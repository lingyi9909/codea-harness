"""Reject missing/skippable Task 2 product gates in the actual workflow graph."""
import argparse
import copy
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
MAIN = ".github/workflows/task164-release-blocker-task2.yml"
REVERIFY = ".github/workflows/task164-release-blocker-task2-reverify-red.yml"
SCRIPTS = {
    "packaged_upgrade": "task164-release-blocker-task2-upgrade-e2e.ps1",
    "top_level_review": "task164-release-blocker-task2-plain-review-e2e.ps1",
    "reviewer_authority": "task164-release-blocker-task2-e2e.ps1",
    "session_cancel": "task164-release-blocker-task2-session-cancel-e2e.ps1",
}
RUNTIME_GATES = {
    "runtime_authority_regression": {
        "working-directory": ".code-harness/tools-runtime",
        "run": "go test -count=1 ./internal/analysis/... ./internal/finding/... ./internal/changeset/... ./internal/coverage/...\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\nWrite-Output 'TASK164_TASK2_RUNTIME_AUTHORITY_REGRESSION PASS'",
    },
    "full_go_test": {
        "working-directory": ".code-harness/tools-runtime",
        "run": "go test -count=1 ./...\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\nWrite-Output 'TASK164_TASK2_FULL_GO_TEST PASS'",
    },
    "go_vet": {
        "working-directory": ".code-harness/tools-runtime",
        "run": "go vet ./...\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\nWrite-Output 'TASK164_TASK2_GO_VET PASS'",
    },
    "scope_exact_head": {
        "run": "pwsh -NoProfile -File ./.github/scripts/task164-release-blocker-task2-scope-self-test.ps1\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\npwsh -NoProfile -File ./.github/scripts/task164-release-blocker-task2-scope.ps1\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }",
    },
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def mandatory(node, label):
    require(isinstance(node, dict), f"{label}: missing")
    for key in ("if", "continue-on-error", "strategy"):
        require(key not in node, f"{label}: conditional/skippable {key}")


def command(script):
    return (
        f"pwsh -NoProfile -File ./.github/scripts/{script}\n"
        "if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }"
    )


def validate(main, reverify):
    require("workflow_call" in main.get("on", {}), "main is not reusable")
    job = main.get("jobs", {}).get("reviewer-host-authority")
    mandatory(job, "main product job")
    require(job.get("runs-on") == "windows-latest", "product E2E requires Windows")
    require(main.get("permissions", {}).get("actions") == "read", "artifact read permission missing")
    steps = job.get("steps", [])
    by_id = {}
    for step in steps:
        if "id" in step:
            require(step["id"] not in by_id, "duplicate gate id")
            by_id[step["id"]] = step
    for gate, script in SCRIPTS.items():
        step = by_id.get(gate)
        mandatory(step, gate)
        require(step.get("shell") == "pwsh", f"{gate}: wrong shell")
        require(step.get("run", "").strip() == command(script), f"{gate}: must execute exact E2E and propagate failure")
        require("uses" not in step and "working-directory" not in step, f"{gate}: unexpected execution override")
    require(by_id["packaged_upgrade"].get("env", {}).get("GH_TOKEN") == "${{ github.token }}", "exact artifact token missing")
    for gate, expected in RUNTIME_GATES.items():
        step = by_id.get(gate)
        mandatory(step, gate)
        require(step.get("shell") == "pwsh", f"{gate}: wrong shell")
        require(step.get("run", "").strip() == expected["run"], f"{gate}: must execute exact validation and propagate failure")
        require(step.get("working-directory") == expected.get("working-directory"), f"{gate}: wrong working directory")
        require("uses" not in step, f"{gate}: unexpected uses override")
    caller = reverify.get("jobs", {}).get("product-gates")
    mandatory(caller, "reverify product-gates")
    require(caller.get("uses") == "./" + MAIN, "reverify must execute current-commit main workflow")
    require(caller.get("needs") == "reverify", "reverify contract must precede product gates")
    require("with" not in caller, "reverify may not opt out of product gates")
    contract = reverify.get("jobs", {}).get("reverify")
    mandatory(contract, "reverify contract")
    checks = [s for s in contract.get("steps", []) if s.get("id") == "gate_contract"]
    require(len(checks) == 1, "reverify gate contract missing")
    mandatory(checks[0], "reverify gate contract")
    expected = "python ./.github/scripts/task164-release-blocker-task2-gate-contract.py --self-test\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }"
    require(checks[0].get("run", "").strip() == expected, "reverify must run structural guard plus negative controls")


def negative_controls(main, reverify):
    cases = []
    for gate in SCRIPTS:
        for mode in ("missing", "skip", "ignore_failure", "fake_pass"):
            m, r = copy.deepcopy(main), copy.deepcopy(reverify)
            steps = m["jobs"]["reviewer-host-authority"]["steps"]
            step = next(s for s in steps if s.get("id") == gate)
            if mode == "missing":
                steps.remove(step)
            elif mode == "skip":
                step["if"] = "false"
            elif mode == "ignore_failure":
                step["continue-on-error"] = "true"
            else:
                step["run"] = "Write-Output 'PASS'"
            cases.append((f"{gate}/{mode}", m, r))
    for gate in RUNTIME_GATES:
        for mode in ("missing", "skip", "ignore_failure", "fake_pass"):
            m, r = copy.deepcopy(main), copy.deepcopy(reverify)
            steps = m["jobs"]["reviewer-host-authority"]["steps"]
            step = next(s for s in steps if s.get("id") == gate)
            if mode == "missing":
                steps.remove(step)
            elif mode == "skip":
                step["if"] = "false"
            elif mode == "ignore_failure":
                step["continue-on-error"] = "true"
            else:
                step["run"] = "Write-Output 'PASS'"
            cases.append((f"{gate}/{mode}", m, r))
    for mode in ("missing", "skip", "stale_workflow"):
        m, r = copy.deepcopy(main), copy.deepcopy(reverify)
        if mode == "missing":
            del r["jobs"]["product-gates"]
        elif mode == "skip":
            r["jobs"]["product-gates"]["if"] = "false"
        else:
            r["jobs"]["product-gates"]["uses"] = "owner/repo/.github/workflows/old.yml@main"
        cases.append((f"reverify/{mode}", m, r))
    for label, m, r in cases:
        try:
            validate(m, r)
        except ValueError:
            continue
        raise ValueError(f"negative control was accepted: {label}")
    print(f"TASK164_TASK2_GATE_REMOVAL_NEGATIVE_CONTROLS PASS cases={len(cases)}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    main = yaml.load((ROOT / MAIN).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
    reverify = yaml.load((ROOT / REVERIFY).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
    validate(main, reverify)
    if args.self_test:
        negative_controls(main, reverify)
    print("TASK164_TASK2_REQUIRED_PRODUCT_GATES PASS")
