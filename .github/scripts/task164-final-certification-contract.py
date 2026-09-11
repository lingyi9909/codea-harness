from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT_PATH = ROOT / ".github/scripts/task164-final-certification.ps1"
WORKFLOW_PATH = ROOT / ".github/workflows/task164-final-certification.yml"
TASK4_REVIEW_PATH = ROOT / ".github/scripts/task164-task4-packaged-plain-review-e2e.ps1"
TASK4_INTERRUPTION_PATH = ROOT / ".github/scripts/task164-task4-progress-interruption-e2e.ps1"

CLOSURE_HOTFIX_BASE = "6605916b4929434ea3362ab5b4fc6325ca117a2f"
OLD_TASK3_BASE = "ffedb2a273dc714db080dc2115f189f20d55b227"
REVOKED_RC = "6aa5d9dad0623cd60a845360c9b20ab153921e87"


def require(text: str, needle: str, label: str, failures: list[str]) -> None:
    if needle not in text:
        failures.append(f"{label}: missing {needle!r}")


def forbid(text: str, needle: str, label: str, failures: list[str]) -> None:
    if needle in text:
        failures.append(f"{label}: forbidden stale token {needle!r}")


def main() -> int:
    script = SCRIPT_PATH.read_text(encoding="utf-8")
    workflow = WORKFLOW_PATH.read_text(encoding="utf-8")
    task4_review = TASK4_REVIEW_PATH.read_text(encoding="utf-8")
    interruption = TASK4_INTERRUPTION_PATH.read_text(encoding="utf-8")
    failures: list[str] = []

    # Closure is a fresh certification above the user-approved hotfix base.
    require(script, f"$base = '{CLOSURE_HOTFIX_BASE}'", "Closure hotfix baseline", failures)
    forbid(script, f"$base = '{OLD_TASK3_BASE}'", "pre-Closure Task 3 baseline", failures)
    require(script, REVOKED_RC, "revoked RC negative fixture", failures)
    require(script, "closureHotfixBase = $base", "Closure checklist baseline", failures)

    # Scope must cover only Host Adapter / Agent Contract / product E2E /
    # install UX / certification files and keep Runtime Go implementation frozen.
    for path in (
        ".code-harness/AGENTS.md",
        ".code-harness/bootstrap.md",
        ".code-harness/agents/orchestrator.md",
        ".code-harness/contracts/reviewer-host-contract.md",
        ".github/scripts/task164-closure-opencode-resolved-contract.ps1",
        ".github/scripts/task164-closure-install-e2e.ps1",
        ".github/scripts/task164-closure-agent-contract.ps1",
        ".github/scripts/task164-install.ps1",
        ".github/scripts/task164-release-package.ps1",
        ".github/scripts/task164-release-blocker-task2-e2e.ps1",
        ".github/scripts/task164-task4-packaged-plain-review-e2e.ps1",
        ".github/scripts/task164-task4-plain-review-server.py",
        ".github/scripts/task164-task4-progress-interruption-e2e.ps1",
        ".github/workflows/task164-closure-product-e2e.yml",
        ".github/workflows/task164-final-certification.yml",
        ".github/scripts/task164-final-certification-contract.py",
        ".github/scripts/task164-final-certification.ps1",
        "README.md",
        "docs/superpowers/plans/2026-09-11-codea-harness-1.6.4-final-certification-closure-hotfix-plan.md",
    ):
        require(script, path, "Closure exact scope allowlist", failures)
    require(script, "Runtime Go implementation changed during Closure", "Runtime freeze guard", failures)

    # OpenCode 1.18.25 resolved Host/command validation must be part of the
    # final exact-head run, not inherited from a prior workflow.
    for path in (
        ".github/scripts/task164-closure-opencode-resolved-contract.ps1",
        ".github/scripts/task164-closure-install-e2e.ps1",
        ".github/scripts/task164-closure-agent-contract.ps1",
    ):
        require(script, path, "Closure preflight gate", failures)
    for marker in (
        "OPENCODE_11825_REVIEWER_PERMISSION_RESOLVED PASS",
        "OPENCODE_11825_REVIEWER_COMMAND_SUBTASK_RESOLVED PASS",
        "REVIEWER_BASH_DENIED PASS",
        "REVIEWER_TASK_DENIED PASS",
        "REVIEWER_RUNTIME_ARTIFACT_WRITE_DENIED PASS",
        "REVIEWER_SUBMIT_TOOL_ALLOWED PASS",
        "INSTALL_REVIEWER_HOST_RESOURCES PASS",
        "INSTALL_EXISTING_OPENCODE_CONFLICT_FAIL_CLOSED PASS",
        "HARNESS_164_AGENT_CONTRACT_CONSISTENT PASS",
    ):
        require(script, marker, "Closure Host/install/contract evidence", failures)

    # Gate A: official packaged 1.6.3 -> exact candidate upgrade remains fresh.
    require(script, ".github/scripts/task164-release-blocker-task1-e2e.ps1", "Gate A packaged upgrade", failures)
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

    # Current Reviewer authority and negative/cancel paths remain mandatory.
    for path in (
        ".github/scripts/task164-release-blocker-task2-plain-review-e2e.ps1",
        ".github/scripts/task164-release-blocker-task2-e2e.ps1",
        ".github/scripts/task164-release-blocker-task2-session-cancel-e2e.ps1",
    ):
        require(script, path, "Reviewer authority E2E", failures)
    for marker in (
        "TASK164_TASK2_TOP_LEVEL_REVIEW_CHAIN PASS",
        "TASK164_TASK2_TOP_LEVEL_DISABLED_HARD_STOP PASS",
        "REVIEWER_INDEPENDENT_INVOCATION PASS",
        "REVIEWER_FINDING_PROPOSAL_HOST_RECEIPT PASS",
        "REVIEWER_RUNTIME_AUTHORITY_SEPARATION PASS",
        "REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS",
        "MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS",
        "MAIN_AGENT_FORGED_REVIEWER_RECEIPT_RUNTIME_REJECTED PASS",
        "REVIEWER_SESSION_CANCEL_FAIL_CLOSED PASS",
        "REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED PASS",
    ):
        require(script, marker, "Reviewer authority evidence", failures)

    # Product Gate B: literal harness review must render Runtime-owned progress.
    require(script, ".github/scripts/task164-task4-packaged-plain-review-e2e.ps1", "positive OpenCode product E2E", failures)
    for marker in (
        "OPENCODE_RUNTIME_PROGRESS_RENDERED PASS",
        "OPENCODE_RUNTIME_PROGRESS_1_TO_8 PASS",
        "PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS",
        "TASK164_TASK4_PACKAGED_PLAIN_REVIEW_8_OF_8 PASS",
        "TASK164_TASK4_INDEPENDENT_REVIEWER_BOTH_PHASES PASS",
        "TASK164_TASK4_RUNTIME_PROGRESS_TERMINAL PASS",
        "TASK164_TASK4_REVIEW_MD PASS",
        "TASK164_TASK4_GATE_B PASS",
    ):
        require(script, marker, "positive OpenCode evidence", failures)
    require(task4_review, "tool_call = $true", "positive custom model tool capability", failures)
    require(interruption, "tool_call = $true", "interruption custom model tool capability", failures)
    forbid(task4_review, "TASK4_STAGE_", "prompt-only positive progress", failures)
    require(task4_review, "events[].display", "Runtime progress source assertion", failures)
    require(task4_review, "forbidden shell orchestration", "root shell contract audit", failures)

    # Product Gate D: interruption must be visible through real OpenCode and
    # stop before all downstream authority/report stages.
    require(script, ".github/scripts/task164-task4-progress-interruption-e2e.ps1", "interruption OpenCode E2E", failures)
    for marker in (
        "OPENCODE_INTERRUPTION_STAGE_VISIBLE PASS",
        "OPENCODE_INTERRUPTION_LATER_STAGES_BLOCKED PASS",
        "TASK164_TASK4_INTERRUPTION_CHANGE_ANALYSIS PASS",
        "TASK164_TASK4_DOWNSTREAM_BLOCKED PASS",
        "TASK164_TASK4_GATE_D PASS",
    ):
        require(script, marker, "interruption OpenCode evidence", failures)
    require(interruption, "REVIEWER_UNAVAILABLE", "interruption failure code", failures)
    require(interruption, "MANUAL_ACTION_REQUIRED", "interruption manual action", failures)
    require(interruption, "HARD STOP", "interruption hard stop", failures)

    # Retained Runtime progress authority plus performance/authority gates.
    require(script, "./internal/reviewprogress", "Task 3 Runtime progress tests", failures)
    require(script, "^Test164Task3", "Task 3 targeted tests", failures)
    for marker in (
        "REVIEW_STAGE_ORDER_RUNTIME_OWNED PASS",
        "REVIEW_STAGE_TRANSITION_FAIL_CLOSED PASS",
        "REVIEW_FRESH_RUN_STATE PASS",
        "REVIEW_STAGE_FAILURE_ATTRIBUTION PASS",
        "OPENCODE_PROGRESS_FROM_RUNTIME_EVENTS PASS",
    ):
        require(script, marker, "Task 3 retained authority", failures)
    require(script, "Test164EntrypointPerformanceWindowsGate", "large-repo performance gate", failures)
    require(script, "^Test164CertifyPerformance", "telemetry performance gate", failures)
    require(script, "Test164VerifyFreshness", "freshness authority gate", failures)
    require(script, "Test153ExplicitTargetUserSelectionPreservesTargetForEveryUpstreamChoice", "USER_SELECTION authority", failures)
    require(script, "./internal/upgrade", "Upgrade V2 retained tests", failures)

    # Fresh whole-product verification and exact package identity are mandatory.
    require(script, "Invoke-Go @('test','-count=1','./...')", "fresh full Go regression", failures)
    require(script, "Invoke-Go @('vet','./...')", "fresh go vet", failures)
    require(script, "install.ps1", "install entrypoint package verification", failures)
    require(script, "TASK164_FINAL_ARTIFACTS PASS", "package hashes", failures)
    require(script, "TASK164_FINAL_EXACT_HEAD PASS", "exact-head guard", failures)

    # Pre-flight workflow must use the same pinned Host contract and execute the
    # contract before the long certification script.
    require(workflow, "actions: read", "Actions read permission", failures)
    require(workflow, "GH_TOKEN: ${{ github.token }}", "official 1.6.3 artifact token", failures)
    require(workflow, "opencode-ai@1.18.25", "pinned OpenCode compatibility", failures)
    require(workflow, "task164-closure-opencode-resolved-contract.ps1", "resolved Host workflow gate", failures)
    require(workflow, "task164-closure-install-e2e.ps1", "install workflow gate", failures)
    require(workflow, "task164-closure-agent-contract.ps1", "Agent contract workflow gate", failures)
    require(workflow, "task164-final-certification-contract.py", "final contract workflow gate", failures)
    require(workflow, "TASK164_FINAL_CONTRACT PASS", "final contract marker", failures)

    # Preserve prior parser/runtime landmines as explicit regressions.
    forbid(task4_review, "[string](if (", "PowerShell inline if regression", failures)
    forbid(script.lower(), "$host =", "PowerShell reserved Host variable", failures)

    if failures:
        print("TASK164_FINAL_CONTRACT FAIL")
        for failure in failures:
            print(f" - {failure}")
        return 1

    print("TASK164_FINAL_CONTRACT PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
