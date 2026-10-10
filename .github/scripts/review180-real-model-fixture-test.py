#!/usr/bin/env python3
"""Fast, model-free contract test for exact CHANGES real-model fixtures.

This test checks REAL changed Java/MyBatis source bytes; it is not a stand-in
for native Host selection or the real-model autonomous acceptance matrix.
"""
import importlib.util
import os
from pathlib import Path
import subprocess
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
        model.write_fixture(project, multi)
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


if __name__ == "__main__":
    unittest.main(verbosity=2)
