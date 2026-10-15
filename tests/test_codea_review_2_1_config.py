"""Codea Review 2.1 P1 static integration regression (stdlib + Git).

Runs locally without TeamAI, OpenCode, network, or a private model.
This does NOT prove the real Windows TUI/LLM has executed knowledge recall.
"""
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ROOT / "codea-review"
BASE_BLOBS = {
    "agents/codea-reviewer.md": "496b584526d883b34358441db12304ae79fa1360",
    "commands/codea-review.md": "bafd6113c571fffd9781f04809dd041fc64ab6d3",
    "skills/codea-review/SKILL.md": "681f90c99c849fc9d81e7dd2293f63f2c27f7b0c",
}


def read(path):
    return (RUNTIME / path).read_text(encoding="utf-8")


def git_blob(path):
    result = subprocess.run(
        ["git", "hash-object", str(RUNTIME / path)],
        cwd=ROOT, check=True, capture_output=True, text=True,
    )
    return result.stdout.strip()


class FrozenReview(unittest.TestCase):
    def test_original_three_files_byte_identical_to_accepted_baseline(self):
        for relative, sha in BASE_BLOBS.items():
            with self.subTest(path=relative):
                self.assertEqual(git_blob(relative), sha)

    def test_original_command_has_no_teamai_injection(self):
        self.assertNotIn("teamai", read("commands/codea-review.md").lower())
        self.assertNotIn("teamai", read("agents/codea-reviewer.md").lower())

    def test_original_skill_retains_git_dirty_and_five_chapters(self):
        text = read("skills/codea-review/SKILL.md")
        for term in (
            "STAGED", "UNSTAGED", "UNTRACKED", "Dirty",
            ":(exclude,glob)**/.*/**", ":(exclude,glob)temp/**",
            "## 评审摘要", "## 调用链与覆盖范围",
            "## 风险清单", "## 问题明细",
            "## 后续处理与评审边界",
            "temp/codea-review-report.md",
        ):
            with self.subTest(term=term):
                self.assertIn(term, text)


class KBExtension(unittest.TestCase):
    def test_one_new_command_points_to_isolated_primary_agent(self):
        command = read("commands/codea-review-kb.md")
        agent = read("agents/codea-reviewer-kb.md")
        self.assertIn("agent: codea-reviewer-kb", command)
        self.assertIn("mode: primary", agent)
        self.assertIn("skill(codea-review)", agent)
        self.assertIn("skill", command)
        self.assertNotIn("skill(codea-review-kb)", command)

    def test_teamai_permissions_are_specific_and_ask_not_wildcard_allow(self):
        agent = read("agents/codea-reviewer-kb.md")
        self.assertIn('resource: "teamai projects list", effect: allow', agent)
        self.assertIn("resource: 'teamai recall \"*\"', effect: ask", agent)
        self.assertNotIn('resource: "teamai *"', agent)
        self.assertNotIn('resource: "teamai recall *", effect: allow', agent)
        self.assertNotIn('resource: "teamai pull", effect: allow', agent)
        self.assertIn('resource: "*;*", effect: deny', agent)
        self.assertIn('resource: "*|*", effect: deny', agent)
        self.assertIn('resource: "*&*", effect: deny', agent)
        self.assertIn('resource: "temp/codea-review-report.md", effect: allow', agent)
        self.assertIn('resource: "*", effect: deny', agent)
        self.assertIn('action: external_directory, resource: "*", effect: deny', agent)

    def test_recall_limits_provenance_and_fallback_are_explicit(self):
        agent = read("agents/codea-reviewer-kb.md")
        for term in (
            "最多两条", "teamai projects list", "[project]", "[user]",
            "File:", "Sources:", "Snippet:", "SCOPE_UNVERIFIED",
            "SOURCE_UNVERIFIED", "TEAMAI_FAILED",
            "继续执行", "本次仅依据代码", "代码证据",
            "不修改", "temp/codea-review-report.md",
        ):
            with self.subTest(term=term):
                self.assertIn(term, agent)

    def test_new_command_has_no_shell_interpolation(self):
        command = read("commands/codea-review-kb.md")
        self.assertIn("$ARGUMENTS", command)
        self.assertIn("绝不将其直接拼进 Shell", command)
        self.assertIn("teamai recall", command)
        self.assertNotIn("!\x60", command)

    def test_a_b_docs_preserve_two_session_design_and_no_fake_results(self):
        docs = (ROOT / "docs/examples/codea-review-2-1-ab-plan.md").read_text(encoding="utf-8")
        self.assertIn("16 次", docs)
        self.assertIn("尚未执行", docs)
        self.assertIn("Copy-Item", docs)
        self.assertIn("S2-A-1", docs)
        self.assertIn("S2-B-1", docs)


if __name__ == "__main__":
    unittest.main(verbosity=2)
