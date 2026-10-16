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
            "默认预算 2 次", "绝对上限 4 次", "teamai projects list", "[project]", "[user]",
            "File:", "Sources:", "Snippet:", "SCOPE_UNVERIFIED",
            "SOURCE_UNVERIFIED", "TEAMAI_FAILED",
            "始终降级继续基础 Review", "本次仅依据代码", "代码证据",
            "禁止修改", "temp/codea-review-report.md",
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



class KnowledgePolicyT1ToT10(unittest.TestCase):
    """Executable *static contract* for P1 policy, NOT real TeamAI/LLM behavior.

    Each corresponding functional scenario must still be run in Windows TUI.
    """

    @classmethod
    def setUpClass(cls):
        cls.agent = read("agents/codea-reviewer-kb.md")
        cls.command = read("commands/codea-review-kb.md")
        cls.documentation = read("KB-README.md")

    def _all(self, *terms):
        for term in terms:
            with self.subTest(term=term):
                self.assertIn(term, self.agent)

    def test_T1_non_behavior_change_skips_recall(self):
        self._all("仅格式", "纯日志文案", "跳过")
        self.assertIn("T1 | 仅日志文案/格式", self.documentation)

    def test_T2_business_change_triggers_recall_without_bug_hypothesis(self):
        self._all("即使尚未产生风险假设", "必须尝试", "业务状态流转")
        self.assertIn("即使尚未发现 Bug", self.command)

    def test_T3_same_business_groups_controller_service_mapper(self):
        self._all("不按文件数、方法数或 Finding 数", "Controller.cancelOrder", "一个场景")
        self.assertIn("业务场景分组", self.command)

    def test_T4_candidates_reuse_knowledge_in_one_session(self):
        self._all("已召回知识表", "同一业务场景中多个候选问题共用", "不允许每个 Finding 重复召回")
        self.assertIn("不逐项重复调用", self.command)

    def test_T5_multiple_scenarios_prioritize_under_four_recall_budget(self):
        self._all("默认预算 2 次", "绝对上限 4 次", "不同独立高风险场景")
        self.assertIn("最多 4 次", self.command)

    def test_T6_generic_java_finding_needs_no_kb(self):
        self._all("KB_NOT_NEEDED", "通用 Java", "不因 TeamAI 没命中而降级")
        self.assertIn("通用 Java", self.command)

    def test_T7_no_hit_and_failure_fall_back(self):
        self._all("无命中", "权限拒绝", "始终降级继续基础 Review", "KB_FAILED")
        self.assertIn("降级", self.command)

    def test_T8_foreign_project_knowledge_not_adopted(self):
        self._all("多项目激活", "无法归属时跳过专属内容", "[project]", "File:", "Sources:")
        self.assertIn("多项目且不能明确归属", self.command)

    def test_T9_conflicted_or_stale_knowledge_not_confirmed(self):
        self._all("过期/冲突", "KB_CONFLICTED", "Source freshness:", "无 Java 证据不得成 Finding")
        self.assertIn("过期/冲突", self.command)

    def test_T10_exhausted_budget_reports_uncovered_but_reviews_code(self):
        self._all("KB_NOT_COVERED", "不得停止代码审查", "未核验的业务场景")
        self.assertIn("预算不足", self.command)

    def test_candidate_knowledge_categories_complete(self):
        self._all("KB_REQUIRED", "KB_USEFUL", "KB_NOT_NEEDED",
                  "KB_SUPPORTED", "KB_CONFLICTED", "KB_NO_MATCH",
                  "KB_NOT_REQUIRED", "KB_NOT_COVERED", "KB_FAILED")

    def test_recall_candidate_cap_and_query_sanitization(self):
        self._all("最多挑选 3 条", "3～6 个", "严禁直接拼接",
                  "原生 Shell", "ask", "teamai projects list")

    def test_p1_docs_do_not_claim_real_model_tests(self):
        self.assertIn("待执行，不是实测", self.documentation)
        self.assertIn("T10", self.documentation)
        docs = (ROOT / "docs/examples/codea-review-2-1-ab-plan.md").read_text(encoding="utf-8")
        self.assertIn("尚未执行", docs)
        self.assertIn("应检索场景覆盖率", docs)
        self.assertIn("知识复用率", docs)
        self.assertIn("Finding 知识核验覆盖率", docs)


if __name__ == "__main__":
    unittest.main(verbosity=2)
