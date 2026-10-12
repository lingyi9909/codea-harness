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


if __name__ == "__main__":
    unittest.main(verbosity=2)
