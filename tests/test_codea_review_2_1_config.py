"""Codea Review 2.1 P1 static integration regression (stdlib + Git).

Runs locally without TeamAI, OpenCode, network, or a private model.
This does NOT prove the real Windows TUI/LLM has executed knowledge recall.
"""
from pathlib import Path
import subprocess
import unittest
import re
import ast
import fnmatch
import tempfile

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


def v2_permission_rules():
    """Parse the narrow single-line V2 permission records (not a general YAML parser).

    This tests ordered glob matching, not OpenCode or OS sandbox correctness.
    """
    front = read("agents/codea-reviewer-kb.md").split("---", 2)[1]
    pattern = re.compile(r'^\s*- \{ action: (.+?), resource: (.+?), effect: (allow|deny|ask) \}$')
    rules = []
    for line in front.splitlines():
        m = pattern.match(line)
        if not m:
            continue
        action, resource, effect = m.groups()
        action = ast.literal_eval(action) if action[:1] in ("'", '"') else action
        resource = ast.literal_eval(resource) if resource[:1] in ("'", '"') else resource
        rules.append((action, resource, effect))
    return rules


def permission_decision(action, resource):
    """Offline approximation of V2 ordered glob rules, not a runtime guarantee."""
    result = "ask"
    for a, r, effect in v2_permission_rules():
        if fnmatch.fnmatchcase(action, a) and fnmatch.fnmatchcase(resource, r):
            result = effect
    return result


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

    def test_v2_native_permission_syntax_and_bounded_access(self):
        agent = read("agents/codea-reviewer-kb.md")
        frontmatter = agent.split("---", 2)[1]
        self.assertIn("\npermissions:\n", frontmatter)
        self.assertNotIn("\npermission:\n", frontmatter)
        self.assertNotIn("action: bash", frontmatter)
        self.assertIn('{ action: "*", resource: "*", effect: deny }', frontmatter)
        self.assertIn('action: external_directory, resource: "*", effect: ask', frontmatter)
        self.assertNotIn('action: external_directory, resource: "*", effect: allow', frontmatter)
        self.assertIn('action: edit, resource: "temp/codea-review-report.md", effect: allow', frontmatter)
        self.assertIn('action: shell, resource: "teamai projects list", effect: allow', frontmatter)
        self.assertIn("""action: shell, resource: 'teamai recall "*"', effect: ask""", frontmatter)
        self.assertNotIn('action: shell, resource: "git *", effect: allow', frontmatter)
        self.assertNotIn('action: shell, resource: "teamai *", effect: allow', frontmatter)
        self.assertEqual(permission_decision("read", "src/main/java/Service.java"), "allow")
        self.assertEqual(permission_decision("read", "C:/TeamAI/knowledge/project-spec.md"), "allow")
        self.assertEqual(permission_decision("external_directory", "C:/TeamAI/knowledge/*"), "ask")
        self.assertEqual(permission_decision("read", ".env"), "deny")
        self.assertEqual(permission_decision("edit", "src/main/java/Service.java"), "deny")
        self.assertEqual(permission_decision("edit", "C:/TeamAI/knowledge/spec.md"), "deny")
        self.assertEqual(permission_decision("edit", "temp/codea-review-report.md"), "allow")
        self.assertEqual(permission_decision("shell", 'teamai recall "订单取消 状态流转规则"'), "ask")
        self.assertEqual(permission_decision("shell", "teamai projects set other-project"), "deny")

    def test_v2_git_four_layer_shell_commands_match_real_pathspecs(self):
        pathspec = "-- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'"
        examples = (
            "git --version",
            "git rev-parse --show-toplevel",
            "git rev-parse HEAD",
            "git rev-parse --abbrev-ref HEAD",
            "git rev-parse origin/master",
            "git rev-parse master",
            "git merge-base origin/master HEAD",
            "git -c core.fsmonitor=false status --porcelain=v1 -z --untracked-files=all",
            "git status --porcelain=v1",
            f"git diff --no-ext-diff --no-textconv --name-status -z 1a2b3c HEAD {pathspec}",
            f"git diff --no-ext-diff --no-textconv --name-status -z --cached HEAD {pathspec}",
            f"git diff --no-ext-diff --no-textconv --name-status -z {pathspec}",
            f"git diff --no-ext-diff --no-textconv --numstat -z HEAD {pathspec}",
            f"git ls-files --others --exclude-standard -z {pathspec}",
            "git diff --no-ext-diff --no-textconv --unified=3 HEAD -- 'src/main/java/demo/OrderService.java'",
            "git diff --no-ext-diff --no-textconv --unified=3 --cached HEAD -- 'src/main/java/demo/OrderService.java'",
            "git diff --no-ext-diff --no-textconv --unified=3 -- 'src/main/java/demo/OrderService.java'",
            "git show --no-ext-diff --no-textconv HEAD:src/main/java/demo/OrderService.java",
            "git show --no-ext-diff --no-textconv :src/main/java/demo/OrderService.java",
        )
        for command in examples:
            with self.subTest(command=command):
                self.assertEqual(permission_decision("shell", command), "allow")

    def test_v2_git_write_and_shell_injection_denied(self):
        commands = (
            "git add -A", "git commit -am update", "git checkout master",
            "git clean -fd", "git reset --hard HEAD", "git push origin main",
            "git fetch --all", "git diff --no-ext-diff --no-textconv --unified=3 --output=tmp.txt",
            "git diff --no-ext-diff --no-textconv --unified=3 HEAD ; git commit -am x",
            "git show --no-ext-diff --no-textconv HEAD | powershell",
            "git merge-base HEAD && git checkout main",
            "git diff --no-ext-diff --no-textconv --unified=3 $(echo x)",
            "git diff --no-ext-diff --no-textconv --unified=3 %TEMP%",
            "teamai push", "teamai projects set payment",
            'teamai recall "x"; teamai push',
            "cmd /c mkdir temp", "powershell New-Item foo",
        )
        for command in commands:
            with self.subTest(command=command):
                self.assertEqual(permission_decision("shell", command), "deny")

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
        frontmatter = command.split("---", 2)[1]
        self.assertRegex(frontmatter, r"(?m)^agent:\s*codea-reviewer-kb\s*$")
        self.assertIn("$ARGUMENTS", command)
        # OpenCode !`...` and shell $() are executable expansions, not prose.
        self.assertNotRegex(command, r"!\s*`")
        self.assertNotRegex(command, r"\$\([^\r\n)]*\$ARGUMENTS")
        # Only inspect code-formatted Git/TeamAI commands. Prohibitive prose
        # can mention $ARGUMENTS without creating command interpolation.
        shell_examples = re.findall(r"`((?:git|teamai)\s+[^`\r\n]*)`", command, re.I)
        self.assertTrue(any(x.startswith("teamai recall ") for x in shell_examples))
        for shell_example in shell_examples:
            with self.subTest(command=shell_example):
                self.assertNotIn("$ARGUMENTS", shell_example)

    def test_a_b_docs_preserve_two_session_design_and_no_fake_results(self):
        docs = (ROOT / "docs/examples/codea-review-2-1-ab-plan.md").read_text(encoding="utf-8")
        self.assertIn("16 次", docs)
        self.assertIn("尚未执行", docs)
        self.assertIn("Copy-Item", docs)
        self.assertIn("S2-A-1", docs)
        self.assertIn("S2-B-1", docs)




class ReviewCompletenessRegression(unittest.TestCase):
    """Static prompt invariants + real Git pathspec checks.

    Does NOT assert an LLM found the three historical production findings.
    """

    def test_generic_and_kb_evidence_must_be_union_not_knowledge_gate(self):
        agent = read("agents/codea-reviewer-kb.md")
        command = read("commands/codea-review-kb.md")
        for clause in (
            "G：通用风险检查轨道", "K：项目知识轨道",
            "最终结果 = G 与 K 的有证据 Finding 并集",
            "检索 TeamAI 前应先生成 G 的候选清单",
            "不得因 K 无命中", "不允许把“TeamAI 没命中”",
            "KB_REQUIRED", "KB_USEFUL", "KB_NOT_NEEDED",
            "最终五章报告", "再次核查 G",
        ):
            with self.subTest(clause=clause):
                self.assertIn(clause, agent)
        self.assertIn("KB 分类不构成通用 G 过滤器", command)
        self.assertIn("报告完整性闸门", command)

    def test_independent_sql_scripts_are_mandatory_without_teamai(self):
        agent = read("agents/codea-reviewer-kb.md")
        command = read("commands/codea-review-kb.md")
        for clause in (
            "独立 .sql 文件作为有效 Git 变更",
            "SQL 仅变更、没有 Java 文件",
            "HEAD/INDEX/WORKTREE/UNTRACKED",
            "UPDATE/DELETE", "DDL", "大小写不敏感",
            "不在根目录", "无需 TeamAI 命中",
            "独立 SQL/迁移脚本；无 Controller 链路",
        ):
            with self.subTest(clause=clause):
                self.assertIn(clause, agent)
        self.assertIn("即使没有 Java 文件", command)
        self.assertIn("SQL untracked 直接 read", command)

    def test_five_chapters_and_no_extra_permissions(self):
        skill = read("skills/codea-review/SKILL.md")
        agent = read("agents/codea-reviewer-kb.md")
        command = read("commands/codea-review-kb.md")
        self.assertIn("五章 Markdown", command)
        self.assertIn("temp/codea-review-report.md", agent)
        for clause in (
            "## 评审摘要", "## 调用链与覆盖范围", "## 风险清单",
            "## 问题明细", "## 后续处理与评审边界",
        ):
            self.assertIn(clause, skill)
        self.assertEqual(permission_decision("shell", "git add -A"), "deny")
        self.assertEqual(permission_decision("shell", "git push origin main"), "deny")
        self.assertEqual(permission_decision("edit", "src/main/java/OrderService.java"), "deny")

    def test_real_git_four_layers_include_standalone_sql_not_hidden_or_temp(self):
        # This is an actual Git fixture with committed, staged, unstaged and
        # untracked .sql changes. It verifies file INVENTORY, not model quality.
        exclude = ":(exclude,glob)**/.*/**"
        exclude_temp = ":(exclude,glob)temp/**"
        scope = ("--", ".", exclude, exclude_temp)

        with tempfile.TemporaryDirectory(prefix="codea-sql-") as working:
            root = Path(working)
            def git(*args):
                cp = subprocess.run(
                    ["git", *args], cwd=root, capture_output=True, check=True,
                )
                return cp.stdout

            def write(relative, data):
                location = root / relative
                location.parent.mkdir(parents=True, exist_ok=True)
                location.write_text(data, encoding="utf-8")

            git("init", "-q")
            git("config", "user.name", "Test")
            git("config", "user.email", "test@example.invalid")
            base_sql = "src/main/resources/db/migration/V1__base.sql"
            committed = "src/main/resources/db/migration/V2__committed.sql"
            staged = "src/main/resources/db/migration/V3__staged.sql"
            untracked = "src/main/resources/db/migration/V4__untracked.sql"
            write(base_sql, "CREATE TABLE accounts(id BIGINT PRIMARY KEY);\n")
            git("add", "--", base_sql)
            git("commit", "-qm", "baseline")
            baseline = git("rev-parse", "HEAD").decode().strip()

            write(committed, "ALTER TABLE accounts ADD COLUMN tenant_id BIGINT;\n")
            git("add", "--", committed)
            git("commit", "-qm", "committed migration")
            write(staged, "UPDATE accounts SET tenant_id = 1;\n")
            git("add", "--", staged)
            write(base_sql, "CREATE TABLE accounts(id BIGINT PRIMARY KEY, amount BIGINT);\n")
            write(untracked, "DELETE FROM accounts;\n")
            write(".teamai/private.sql", "DELETE FROM accounts;\n")
            write("src/.agents/private.sql", "DELETE FROM accounts;\n")
            write("temp/audit.sql", "DELETE FROM accounts;\n")

            from_base = git(
                "diff", "--no-ext-diff", "--no-textconv",
                "--name-status", "-z", baseline, "HEAD", *scope,
            )
            index = git(
                "diff", "--no-ext-diff", "--no-textconv",
                "--name-status", "-z", "--cached", "HEAD", *scope,
            )
            working_diff = git(
                "diff", "--no-ext-diff", "--no-textconv",
                "--name-status", "-z", *scope,
            )
            added = git(
                "ls-files", "--others", "--exclude-standard", "-z", *scope,
            )
            expected = [committed, staged, base_sql, untracked]
            for src, name in zip((from_base, index, working_diff, added), expected):
                with self.subTest(layer=name):
                    self.assertIn(name.encode("utf-8") + b"\0", src)
                    self.assertNotIn(b"private.sql", src)
                    self.assertNotIn(b"temp/audit.sql", src)
            self.assertNotIn(b"temp/audit.sql", added)

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
        agent = read("agents/codea-reviewer-kb.md")
        phase = agent.split("### 第二阶段：候选 Finding 逐项核验", 1)[1]
        phase = phase.split("### 查询预算与失败降级", 1)[0]
        # A conflicted project rule must remain unconfirmed, not become
        # a knowledge-only Finding; verify the decision gate, not wording.
        required = next(line for line in phase.splitlines() if "`KB_REQUIRED`" in line)
        self.assertIn("冲突", required)
        self.assertIn("待确认", required)
        self.assertIn("KB_CONFLICTED", phase)
        evidence = next(line for line in phase.splitlines() if line.startswith("11."))
        for requirement in ("来源", "当前有效", "冲突", "变更", "证据", "等效保护"):
            with self.subTest(requirement=requirement):
                self.assertIn(requirement, evidence)
        self.assertIn("Source freshness:", agent)

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
