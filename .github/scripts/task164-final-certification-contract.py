from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT_PATH = ROOT / ".github/scripts/task164-final-certification.ps1"
WORKFLOW_PATH = ROOT / ".github/workflows/task164-final-certification.yml"

ACCEPTED_TASK3 = "ffedb2a273dc714db080dc2115f189f20d55b227"


def require(text: str, needle: str, label: str, failures: list[str]) -> None:
    if needle not in text:
        failures.append(f"{label}: missing {needle!r}")


def forbid(text: str, needle: str, label: str, failures: list[str]) -> None:
    if needle in text:
        failures.append(f"{label}: forbidden stale token {needle!r}")


def main() -> int:
    script = SCRIPT_PATH.read_text(encoding="utf-8")
    workflow = WORKFLOW_PATH.read_text(encoding="utf-8")
    failures: list[str] = []

    # Final certification freezes the accepted repair Task 3 HEAD. Task 4 may
    # change only final-certification/evidence code above that frozen baseline.
    require(script, f"$base = '{ACCEPTED_TASK3}'", "accepted Task 3 baseline", failures)
    forbid(script, "$base = '48158a74a5cbec61ac8936e1c65901013e757101'", "stale final baseline", failures)

    # Gate A: official packaged 1.6.3 -> candidate upgrade with migration,
    # preservation, idempotency and fail-closed evidence on the final HEAD.
    require(script, ".github/scripts/task164-release-blocker-task1-e2e.ps1", "Gate A packaged upgrade E2E", failures)
    for marker in (
        "CONFIG_MIGRATION_163_TO_164_REGISTERED PASS",
        "CONFIG_MIGRATION_BEFORE_TARGET_SCHEMA_VALIDATION PASS",
        "CONFIG_MIGRATION_TARGET_SCHEMA_VALID PASS",
        "CONFIG_MIGRATION_IDEMPOTENT PASS",
        "CONFIG_USER_VALUES_PRESERVED PASS",
        "CONFIG_UNSUPPORTED_MIGRATION_FAIL_CLOSED PASS",
        "TASK164_CONFIG_PACKAGED_163_TO_164_E2E PASS",
    ):
        require(script, marker, "Gate A evidence", failures)

    # Gates B/C: packaged OpenCode positive independent Reviewer authority plus
    # Reviewer-unavailable/cancel fail-closed paths must be rerun on final HEAD.
    for path in (
        ".github/scripts/task164-release-blocker-task2-plain-review-e2e.ps1",
        ".github/scripts/task164-release-blocker-task2-e2e.ps1",
        ".github/scripts/task164-release-blocker-task2-session-cancel-e2e.ps1",
        ".github/scripts/task164-task4-packaged-plain-review-e2e.ps1",
    ):
        require(script, path, "Gate B/C Reviewer E2E", failures)
    for marker in (
        "TASK164_TASK2_TOP_LEVEL_REVIEW_CHAIN PASS",
        "TASK164_TASK2_TOP_LEVEL_DISABLED_HARD_STOP PASS",
        "TASK164_RELEASE_BLOCKER_TASK2_E2E PASS",
        "TASK164_TASK4_PACKAGED_PLAIN_REVIEW_8_OF_8 PASS",
        "TASK164_TASK4_INDEPENDENT_REVIEWER_BOTH_PHASES PASS",
        "TASK164_TASK4_RUNTIME_PROGRESS_TERMINAL PASS",
        "TASK164_TASK4_REVIEW_MD PASS",
        "TASK164_TASK4_GATE_B PASS",
    ):
        require(script, marker, "Gate B/C evidence", failures)

    # Gate D: a controlled packaged interruption must fail the exact Runtime
    # stage and prove all later stages are BLOCKED, without review.md.
    require(script, ".github/scripts/task164-task4-progress-interruption-e2e.ps1", "Gate D interruption E2E", failures)
    for marker in (
        "TASK164_TASK4_INTERRUPTION_CHANGE_ANALYSIS PASS",
        "TASK164_TASK4_DOWNSTREAM_BLOCKED PASS",
        "TASK164_TASK4_GATE_D PASS",
    ):
        require(script, marker, "Gate D evidence", failures)

    # Task 3 authority is re-run, not inherited from a previous green run.
    require(script, "./internal/reviewprogress", "Task 3 Runtime progress tests", failures)
    require(script, "^Test164Task3", "Task 3 targeted tests", failures)
    for marker in (
        "REVIEW_STAGE_ORDER_RUNTIME_OWNED PASS",
        "REVIEW_STAGE_TRANSITION_FAIL_CLOSED PASS",
        "REVIEW_FRESH_RUN_STATE PASS",
        "REVIEW_STAGE_FAILURE_ATTRIBUTION PASS",
        "OPENCODE_PROGRESS_FROM_RUNTIME_EVENTS PASS",
        "PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS",
    ):
        require(script, marker, "Task 3 final-head evidence", failures)

    # Retained full product review/report regression and final exact-head gates
    # remain part of certification. These assertions follow the production
    # wrapper used by this script instead of depending on an implementation
    # detail inside Invoke-Go itself.
    require(script, ".github/scripts/task162-hotfix-task3-real-plain-review-e2e.ps1", "full review.md regression", failures)
    require(script, "Invoke-Go @('test','-count=1','./...')", "fresh full Go regression", failures)
    require(script, "Invoke-Go @('vet','./...')", "fresh go vet", failures)
    require(script, "TASK164_FINAL_ARTIFACTS PASS", "release artifact hashes", failures)
    require(script, "TASK164_FINAL_EXACT_HEAD PASS", "final exact HEAD", failures)

    # The workflow must enforce the contract before the long run and provide
    # Actions read access / token needed for the official 1.6.3 artifact fetch.
    require(workflow, "task164-final-certification-contract.py", "workflow contract gate", failures)
    require(workflow, "TASK164_FINAL_CONTRACT PASS", "workflow contract marker", failures)
    require(workflow, "actions: read", "official artifact Actions permission", failures)
    require(workflow, "GH_TOKEN: ${{ github.token }}", "official artifact token", failures)
    require(workflow, "task164-task4-packaged-plain-review-e2e.ps1", "Task 4 PowerShell parse gate", failures)
    require(workflow, "task164-task4-progress-interruption-e2e.ps1", "Task 4 interruption parse gate", failures)
    require(workflow, "task164-task4-plain-review-server.py", "Task 4 model py_compile gate", failures)

    if failures:
        print("TASK164_FINAL_CONTRACT FAIL")
        for failure in failures:
            print(f" - {failure}")
        return 1

    print("TASK164_FINAL_CONTRACT PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
