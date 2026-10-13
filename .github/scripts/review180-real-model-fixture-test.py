#!/usr/bin/env python3
"""Fast, model-free contract test for exact CHANGES real-model fixtures.

This test checks REAL changed Java/MyBatis source bytes; it is not a stand-in
for native Host selection or the real-model autonomous acceptance matrix.
"""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parent / "review180-real-model-e2e.py"
SPEC = importlib.util.spec_from_file_location("release180_model", SCRIPT)
model = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(model)


def git(project, *args):
    completed = subprocess.run(["git", *args], cwd=project, check=True,
                               capture_output=True, text=True, encoding="utf-8")
    return completed.stdout


class ExactImpactedChainFixtureTest(unittest.TestCase):
    def fixture(self, multi, scenario):
        temp = tempfile.TemporaryDirectory(prefix="Codea fixture proof ")
        self.addCleanup(temp.cleanup)
        project = Path(temp.name)
        model.write_fixture(project, multi, multiple_controllers=(scenario == "two-chains-select-c1"))
        git(project, "init", "--quiet")
        git(project, "config", "core.autocrlf", "false")
        git(project, "add", ".")
        git(project, "-c", "user.name=Fixture Contract",
            "-c", "user.email=fixture@codea.invalid", "commit", "--quiet",
            "-m", "baseline")
        model.mutate_for_scenario(project, scenario)
        java = project / "src/main/java/com/example/OrderController.java"
        mapper = project / "src/main/resources/mapper/OrderMapper.xml"
        diff = git(project, "diff", "--unified=0", "--",
                   "src/main/java/com/example/OrderController.java",
                   "src/main/resources/mapper/OrderMapper.xml")
        return java.read_text(encoding="utf-8"), mapper.read_text(encoding="utf-8"), diff

    def test_windows_cp1252_console_can_log_unicode_controller_names(self):
        # Reproduce the real CI #192 traceback without a model invocation.
        # The script itself must set UTF-8 before it logs options/proofs.
        env = dict(os.environ)
        env["PYTHONIOENCODING"] = "cp1252"
        code = (
            "import importlib.util; "
            f"spec=importlib.util.spec_from_file_location('release180_log_check', {str(SCRIPT)!r}); "
            "m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m); "
            "print('C1 OrderController（3 个受影响方法）')"
        )
        proc = subprocess.run([sys.executable, "-c", code], env=env,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              cwd=SCRIPT.parent, check=False)
        self.assertEqual(proc.returncode, 0, proc.stderr.decode("utf-8", "replace"))
        self.assertIn("OrderController（3 个受影响方法）",
                      proc.stdout.decode("utf-8"))

    def test_two_independent_changed_chain_sql_statements(self):
        controller, mapper, diff = self.fixture(True, "two-chains-select-c1")
        self.assertIn("service.updateStatus(tenantId, id, status);", controller)
        self.assertIn("service.voidOrder(tenantId, id);", controller)
        self.assertIn(model.RISKY_SQL, mapper)
        self.assertIn("voidOrder", mapper)
        self.assertIn("status = 'VOIDED' WHERE id = #{id} AND tenant_id = #{tenantId}", mapper)
        self.assertIn("+  <update id=\"updateStatus\">", diff)
        self.assertIn("+  <update id=\"voidOrder\">", diff)
        self.assertIn("-  <update id=\"voidOrder\">", diff)
        self.assertNotIn("tenant_id = #{tenantId} AND status = #{expectedStatus}", diff.split("+  <update id=\"voidOrder\">")[-1].split("\n")[0])
        self.assertIn("AND tenant_id = #{tenantId}</update>", diff)

    def test_two_endpoints_only_one_changed_chain(self):
        _, mapper, diff = self.fixture(True, "single-affected-two-endpoint")
        self.assertIn(model.RISKY_SQL, mapper)
        self.assertIn("status = 'CANCELLED' WHERE id = #{id} AND tenant_id = #{tenantId}", mapper)
        self.assertIn("+  <update id=\"updateStatus\">", diff)
        self.assertNotIn("+  <update id=\"voidOrder\">", diff)
        self.assertNotIn("-  <update id=\"voidOrder\">", diff)

    def test_single_issue_does_not_invent_void_endpoint(self):
        controller, mapper, diff = self.fixture(False, "single-issue")
        self.assertNotIn("voidOrder", controller)
        self.assertNotIn("voidOrder", mapper)
        self.assertIn("+  <update id=\"updateStatus\">", diff)

    def test_agent_evidence_quote_contract_is_consistent(self):
        # CI #195: a model dedented a multiline Java quote, then retried with
        # corrected line numbers only. Assert all three shipped instructions
        # steer it to a literal single physical source line instead.
        root = SCRIPT.parents[2]
        for path in (
            ".code-harness/tools/codea-review.ts",
            ".code-harness/commands/harness-review.md",
            ".code-harness/agents/orchestrator.md",
        ):
            with self.subTest(path=path):
                text = (root / path).read_text(encoding="utf-8")
                self.assertIn("single physical source line", text)
                self.assertIn("REVIEW_FINISH_EVIDENCE_QUOTE_MISMATCH", text)


    def test_strict_scope_tool_configuration_and_attempt_audit(self):
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertNotIn('config["tools"]', source)
        self.assertNotIn('"glob": "allow"', source)
        self.assertIn("Shipped Review Agent MUST enforce the", source)
        ref = {"path": "src/main/java/A.java", "sha256": "a" * 64, "startLine": 1, "endLine": 3}
        scope = {"reads": [ref]}
        def trace(calls):
            return {"messages": [{"parts": [
                {"type": "tool", "tool": name, "state": {"input": args, "status": status}}
                for name, args, status in calls]}]}
        good = trace([("codea-review", {"action": "prepare"}, "completed"),
                      ("codea-review", {"action": "read", "source": ref}, "completed"),
                      ("codea-review", {"action": "finish"}, "completed")])
        self.assertEqual(model.audit_native_review_tools(good, scope)["violations"], [])
        glob = trace([("codea-review", {"action": "prepare"}, "completed"),
                      ("codea-review", {"action": "read", "source": ref}, "completed"),
                      ("glob", {"pattern": "**/*"}, "completed"),
                      ("codea-review", {"action": "finish"}, "completed")])
        self.assertTrue(any("forbidden tool" in v for v in
                            model.audit_native_review_tools(glob, scope)["violations"]))
        retry = trace([("codea-review", {"action": "prepare"}, "completed"),
                       ("codea-review", {"action": "read", "source": ref}, "completed"),
                       ("codea-review", {"action": "finish"}, "error"),
                       ("codea-review", {"action": "finish"}, "completed")])
        self.assertTrue(any("non-completed" in v for v in
                            model.audit_native_review_tools(retry, scope)["violations"]))
        unrelated = trace([("codea-review", {"action": "prepare"}, "completed"),
                           ("codea-review", {"action": "read", "source": dict(ref, path="B.java")}, "completed"),
                           ("codea-review", {"action": "finish"}, "completed")])
        self.assertTrue(any("unauthorized source" in v for v in
                            model.audit_native_review_tools(unrelated, scope)["violations"]))
        incomplete = trace([("codea-review", {"action": "prepare"}, "completed"),
                            ("codea-review", {"action": "read", "source": ref}, "completed")])
        self.assertTrue(model.audit_native_review_tools(incomplete, scope)["violations"])

    def test_formal_package_agent_isolation_contract(self):
        root = SCRIPT.parents[2]
        agent = (root / ".code-harness/agents/codea-review-restricted.md").read_text(encoding="utf-8")
        cmd = (root / ".code-harness/commands/harness-review.md").read_text(encoding="utf-8")
        host = (root / ".github/scripts/review180-host-smoke.py").read_text(encoding="utf-8")
        for marker in ('mode: primary', '"*": deny', '"codea-review": allow'):
            self.assertIn(marker, agent)
        self.assertIn("agent: codea-review-restricted", cmd)
        self.assertIn("run_forbidden_builtin_scenario", host)
        self.assertIn("run_orchestrator_unrestricted_scenario", host)
        # The OpenCode title model always receives tools=[], which is not
        # evidence of a restricted orchestrator. Require a real tool request.
        self.assertIn('if self.server.scenario == "orchestrator-permission-check":', host)
        self.assertIn('if not names:', host)
        self.assertIn('and req["names"]', host)
        self.assertIn('require(tool_requests,', host)
        self.assertNotIn('"tools": {"read": False', host)
        self.assertIn('"--install-zip"', host)
        # All installed OpenCode custom tools must load offline, even legacy
        # tools that the restricted Review agent cannot invoke.
        pkg = (root / ".github/scripts/release180-package.ps1").read_text(encoding="utf-8")
        workflow = (root / ".github/workflows/release-1.8.0-windows-x64.yml").read_text(encoding="utf-8")
        self.assertIn("codea-reviewer-submit.offline.ts", pkg)
        self.assertIn("submissionToolSha256", pkg)
        self.assertIn("codea-reviewer-submit.offline.ts", workflow)
        self.assertIn("REVIEW180_LEGACY_HOST_OFFLINE_BUNDLE PASS", workflow)

    def test_native_read_tool_rejects_unscoped_access(self):
        tool = (SCRIPT.parents[2] / ".code-harness/tools/codea-review.ts").read_text(encoding="utf-8")
        for control in (
            "CODEA_REVIEW_SOURCE_OUTSIDE_SCOPE", "CODEA_REVIEW_SOURCE_STALE",
            "CODEA_REVIEW_SOURCE_SCOPE_NOT_READY", "CODEA_REVIEW_SOURCE_PATH_INVALID",
            'action: tool.schema.enum(["prepare", "read", "finish"])',
            "createHash", "realpath",
        ):
            self.assertIn(control, tool)


    def test_ci201_line_evidence_eligibility_contract(self):
        # CI #201: source reads cover entire files, but a quoted Controller import
        # from line 2 is not method-scoped evidence and must not be selectable.
        root = SCRIPT.parents[2]
        native = (root / ".code-harness/tools/codea-review.ts").read_text(encoding="utf-8")
        host_smoke = (root / ".github/scripts/review180-host-smoke.py").read_text(encoding="utf-8")
        for required in ("scope.findingReads", "findingRanges", "evidenceEligible",
                         "CODEA_REVIEW_FINDING_SCOPE_NOT_READY"):
            self.assertIn(required, native)
        self.assertIn('row.get("evidenceEligible") is expected', host_smoke)
        for agent in (".code-harness/commands/harness-review.md",
                      ".code-harness/agents/orchestrator.md"):
            with self.subTest(agent=agent):
                md = (root / agent).read_text(encoding="utf-8")
                self.assertIn("scope.findingReads", md)
                self.assertIn("evidenceEligible", md)
                self.assertIn("不能引用", md)

    def test_ci200_introduced_by_change_guidance(self):
        # CI #200: real model incorrectly called an unchanged Controller
        # authorization flaw newly introduced by changed downstream SQL.
        root = SCRIPT.parents[2]
        for path in (
            ".code-harness/tools/codea-review.ts",
            ".code-harness/commands/harness-review.md",
            ".code-harness/agents/orchestrator.md",
        ):
            with self.subTest(path=path):
                content = (root / path).read_text(encoding="utf-8")
                self.assertIn("introducedByChange=true requires proof", content)
                self.assertIn("introducedByChange=false", content)
        for path in (
            ".code-harness/commands/harness-review.md",
            ".code-harness/agents/orchestrator.md",
        ):
            self.assertIn("REVIEW_FINISH_CHANGE_ATTRIBUTION_UNPROVEN",
                          (root / path).read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main(verbosity=2)
