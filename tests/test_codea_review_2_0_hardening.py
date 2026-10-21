"""Codea Review 2.0 hardening: deterministic Git, permissions, and report contracts.

No model run here: real Windows OpenCode + private model Finding accuracy is separate.
"""
from __future__ import annotations

import ast
import fnmatch
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
AGENT = ROOT / "codea-review/agents/codea-reviewer.md"
COMMAND = ROOT / "codea-review/commands/codea-review.md"
SKILL = ROOT / "codea-review/skills/codea-review/SKILL.md"

# Same two pathspecs as SKILL.md: exclusion is done by Git, BEFORE reading files.
EXCLUDES = (":(exclude,glob)**/.*/**", ":(exclude,glob)temp/**")
SCOPE = ("--", ".", *EXCLUDES)
LAYERS = ("COMMITTED", "STAGED", "UNSTAGED", "UNTRACKED")


def git(root: Path, *args: str) -> bytes:
    cmd = ("git", *args)
    result = subprocess.run(cmd, cwd=root, check=True, capture_output=True)
    return result.stdout


def write(root: Path, path: str, data: str) -> None:
    p = root / path
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(data, encoding="utf-8")


def git_inventory(root: Path, base: str) -> dict[str, bytes]:
    # Commands intentionally identical to the existing Skill's four listing forms.
    return {
        "COMMITTED": git(root, "diff", "--no-ext-diff", "--no-textconv",
                         "--name-status", "-z", base, "HEAD", *SCOPE),
        "STAGED": git(root, "diff", "--no-ext-diff", "--no-textconv",
                      "--name-status", "-z", "--cached", "HEAD", *SCOPE),
        "UNSTAGED": git(root, "diff", "--no-ext-diff", "--no-textconv",
                        "--name-status", "-z", *SCOPE),
        "UNTRACKED": git(root, "ls-files", "--others", "--exclude-standard",
                         "-z", *SCOPE),
    }


def parsed_git_entries(layer: str, output: bytes) -> list[tuple[str, str, str | None]]:
    """Test-only decoder of actual NUL-delimited Git inventory output.

    Name-status -z: status\0path\0, or R/C status\0old\0new\0;
    untracked ls-files -z: path\0. Canonical identity is the NEW path.
    """
    if not output:
        return []
    assert output.endswith(b"\0"), "truncated Git -z output"
    fields = output[:-1].split(b"\0")
    out: list[tuple[str, str, str | None]] = []
    i = 0
    while i < len(fields):
        if layer == "UNTRACKED":
            out.append(("?", fields[i].decode("utf-8", "surrogateescape"), None))
            i += 1
            continue
        status = fields[i].decode("ascii")
        assert re.fullmatch(r"(?:[ACDMUTX]|R\d{0,3}|C\d{0,3})", status), status
        i += 1
        if status.startswith(("R", "C")):
            assert i + 1 < len(fields), "truncated rename/copy"
            old = fields[i].decode("utf-8", "surrogateescape")
            new = fields[i + 1].decode("utf-8", "surrogateescape")
            out.append((status, new, old))
            i += 2
        else:
            assert i < len(fields), "missing Git path"
            out.append((status, fields[i].decode("utf-8", "surrogateescape"), None))
            i += 1
    return out


def unique_paths_from_real_git(results: dict[str, bytes]) -> dict[str, set[str]]:
    """Simple set-based accounting for the TEST fixture, not a Review Runtime."""
    merged: dict[str, set[str]] = {}
    for layer, output in results.items():
        for _status, path, _old in parsed_git_entries(layer, output):
            merged.setdefault(path, set()).add(layer)
    return merged


def parse_v2_permissions() -> list[tuple[str, str, str]]:
    front = AGENT.read_text(encoding="utf-8").split("---", 2)[1]
    rules = []
    row = re.compile(r"^\s*- \{ action: (.+?), resource: (.+?), effect: (allow|deny|ask) \}$")
    for line in front.splitlines():
        m = row.match(line)
        if not m:
            continue
        a, resource, effect = m.groups()
        decode = lambda x: ast.literal_eval(x) if x[:1] in ("'", '"') else x
        rules.append((decode(a), decode(resource), effect))
    return rules


def permission(action: str, resource: str) -> str:
    """Approximate ordered OpenCode V2 glob rules; not a real TUI decision."""
    decision = "ask"
    for a, r, effect in parse_v2_permissions():
        if fnmatch.fnmatchcase(action, a) and fnmatch.fnmatchcase(resource, r):
            decision = effect
    return decision


class SQLGitScope(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="codea-2-sql-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        git(self.root, "init", "-q", "-b", "master")
        git(self.root, "config", "user.name", "Review Test")
        git(self.root, "config", "user.email", "test@example.invalid")

        self.shared = "src/main/resources/db/migration/V1__overlap.SQL"
        self.committed = "src/main/resources/db/migration/V2__committed.sql"
        self.staged = "db/migration/V3__staged.sql"
        self.unstaged = "database/V4__unstaged.SQL"
        self.untracked = "src/main/resources/sql/V5__untracked.sql"
        self.java = "src/main/java/example/OrderService.java"
        write(self.root, self.shared, "CREATE TABLE orders (id BIGINT PRIMARY KEY);\n")
        write(self.root, self.unstaged, "UPDATE orders SET id = id WHERE id = 1;\n")
        git(self.root, "add", "--", self.shared, self.unstaged)
        git(self.root, "commit", "-qm", "baseline")
        self.base = git(self.root, "rev-parse", "HEAD").decode().strip()

        # Committed SQL; one file later also appears in STAGED and UNSTAGED.
        write(self.root, self.shared, "CREATE TABLE orders (id BIGINT PRIMARY KEY, state INT);\n")
        write(self.root, self.committed, "ALTER TABLE orders ADD COLUMN tenant_id BIGINT;\n")
        git(self.root, "add", "--", self.shared, self.committed)
        git(self.root, "commit", "-qm", "committed SQL")

        write(self.root, self.shared, "CREATE TABLE orders (id BIGINT PRIMARY KEY, state INT, amount BIGINT);\n")
        write(self.root, self.staged, "UPDATE orders SET amount = 10 WHERE id = 1;\n")
        git(self.root, "add", "--", self.shared, self.staged)

        write(self.root, self.shared, "CREATE TABLE orders (id BIGINT PRIMARY KEY, state INT, amount BIGINT, remark TEXT);\n")
        write(self.root, self.unstaged, "UPDATE orders SET id = id WHERE id = 2;\n")
        write(self.root, self.untracked, "DELETE FROM orders WHERE id = 99;\n")
        write(self.root, ".teamai/secrets.sql", "DELETE FROM orders;\n")
        write(self.root, "src/.opencode/ignore.SQL", "DELETE FROM orders;\n")
        write(self.root, "temp/codea-review-report.md", "report\n")
        write(self.root, "temp/unsafe.sql", "DELETE FROM orders;\n")

    def test_all_four_git_layers_and_case_insensitive_sql(self):
        results = git_inventory(self.root, self.base)
        expected = {
            "COMMITTED": [self.shared, self.committed],
            "STAGED": [self.shared, self.staged],
            "UNSTAGED": [self.shared, self.unstaged],
            "UNTRACKED": [self.untracked],
        }
        for layer, paths in expected.items():
            with self.subTest(layer=layer):
                self.assertTrue(results[layer], f"{layer} missing")
                for path in paths:
                    self.assertIn(path.encode() + b"\0", results[layer])
                for excluded in (b".teamai/", b".opencode/", b"temp/"):
                    self.assertNotIn(excluded, results[layer])
        self.assertEqual({x for paths in expected.values() for x in paths},
                         {self.shared, self.committed, self.staged, self.unstaged, self.untracked})

    def test_same_sql_file_across_three_layers_is_one_effective_file(self):
        results = git_inventory(self.root, self.base)
        # Parse REAL Git -z bytes, deduplicate by canonical repo-relative path,
        # and retain all provenance layers for the overlapping SQL script.
        actual = unique_paths_from_real_git(results)
        expected = {
            self.shared: {"COMMITTED", "STAGED", "UNSTAGED"},
            self.committed: {"COMMITTED"},
            self.staged: {"STAGED"},
            self.unstaged: {"UNSTAGED"},
            self.untracked: {"UNTRACKED"},
        }
        self.assertEqual(actual, expected)
        self.assertEqual(len(actual), 5)
        self.assertEqual(sum(len(v) for v in actual.values()), 7)
        self.assertEqual(
            len([x for x in actual if x.lower().endswith(".sql")]), 5
        )

    def test_git_rename_decodes_old_and_new_path_without_double_counting(self):
        renamed = "src/main/resources/db/migration/V2__renamed.sql"
        git(self.root, "mv", self.committed, renamed)
        results = git_inventory(self.root, self.base)
        entries = parsed_git_entries("STAGED", results["STAGED"])
        renames = [(status, path, old) for status, path, old in entries
                   if status.startswith("R")]
        self.assertIn(("R100", renamed, self.committed), renames)
        actual = unique_paths_from_real_git(results)
        self.assertIn(renamed, actual)
        self.assertNotIn(self.committed, [path for status, path, old in entries])
        self.assertEqual(actual[renamed], {"STAGED"})

    def test_sql_only_and_mixed_java_sql(self):
        only_sql = git_inventory(self.root, self.base)
        self.assertTrue(any(only_sql.values()))
        self.assertNotIn(b".java\0", b"".join(only_sql.values()))
        write(self.root, self.java, "package example; class OrderService {}\n")
        mixed = git_inventory(self.root, self.base)
        self.assertIn(self.java.encode() + b"\0", mixed["UNTRACKED"])
        self.assertIn(self.untracked.encode() + b"\0", mixed["UNTRACKED"])

    def test_untracked_sql_is_real_worktree_content(self):
        results = git_inventory(self.root, self.base)
        self.assertIn(self.untracked.encode() + b"\0", results["UNTRACKED"])
        actual = (self.root / self.untracked).read_text(encoding="utf-8")
        self.assertIn("DELETE FROM orders WHERE id = 99", actual)
        self.assertNotIn(b"temp/codea-review-report.md", results["UNTRACKED"])

    def test_diff_context_and_index_are_distinct(self):
        committed = git(self.root, "diff", "--no-ext-diff", "--no-textconv",
                        "--unified=3", self.base, "HEAD", "--", self.shared).decode()
        staged = git(self.root, "diff", "--no-ext-diff", "--no-textconv",
                     "--unified=3", "--cached", "HEAD", "--", self.shared).decode()
        unstaged = git(self.root, "diff", "--no-ext-diff", "--no-textconv",
                       "--unified=3", "--", self.shared).decode()
        self.assertIn("state INT", committed)
        self.assertIn("amount BIGINT", staged)
        self.assertIn("remark TEXT", unstaged)
        index_snapshot = git(self.root, "show", "--no-ext-diff", "--no-textconv",
                             ":" + self.shared).decode()
        self.assertIn("amount BIGINT", index_snapshot)
        self.assertNotIn("remark TEXT", index_snapshot)


class PermissionsAndReport(unittest.TestCase):
    """Tool-level permission contracts, not an OpenCode live sandbox test."""

    def test_original_entry_and_v2_format(self):
        agent = AGENT.read_text(encoding="utf-8")
        command = COMMAND.read_text(encoding="utf-8")
        self.assertIn("\npermissions:\n", agent)
        self.assertNotIn("\npermission:\n", agent)
        self.assertIn("agent: codea-reviewer", command)
        self.assertIn("codea-review", command)
        self.assertNotIn("teamai recall", command.lower())
        self.assertFalse(any(a == "shell" and r.startswith("teamai ")
                             for a, r, _ in parse_v2_permissions()))

    def test_shell_policy_has_safe_exact_git_allows(self):
        rules = parse_v2_permissions()
        shell = [(r, effect) for action, r, effect in rules if action == "shell"]
        self.assertEqual(shell[0], ("*", "ask"))
        self.assertEqual(rules[0], ("*", "*", "deny"))
        self.assertNotIn(("git *", "allow"), shell)
        for pattern, effect in shell:
            if effect == "allow":
                with self.subTest(pattern=pattern):
                    self.assertTrue(pattern.startswith("git "))
                    self.assertNotIn("*", pattern)
                    self.assertNotIn("?", pattern)

    def test_fixed_git_read_queries_allow(self):
        commands = (
            "git --version", "git status", "git status --short",
            "git status --porcelain=v1",
            "git -c core.fsmonitor=false status --porcelain=v1 -z --untracked-files=all",
            "git rev-parse --show-toplevel", "git rev-parse HEAD",
            "git rev-parse --abbrev-ref HEAD",
            "git rev-parse origin/master", "git rev-parse master",
            "git merge-base origin/master HEAD", "git merge-base master HEAD",
            "git log -1 --oneline",
            "git show --no-ext-diff --no-textconv --stat HEAD",
            "git diff --no-ext-diff --no-textconv --stat",
        )
        for command in commands:
            with self.subTest(command=command):
                self.assertEqual(permission("shell", command), "allow")

    def test_dynamic_git_read_queries_and_other_shell_ask(self):
        scope = "-- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'"
        commands = (
            f"git diff --no-ext-diff --no-textconv --name-status -z abc HEAD {scope}",
            f"git diff --no-ext-diff --no-textconv --name-status -z --cached HEAD {scope}",
            f"git diff --no-ext-diff --no-textconv --name-status -z {scope}",
            f"git ls-files --others --exclude-standard -z {scope}",
            "git show --no-ext-diff --no-textconv HEAD:database/V2__migration.sql",
            "git blame -L 2,40 -- src/main/java/OrderService.java",
            "git grep -n -- tenant_id", "git log --oneline -10",
            "git diff --no-ext-diff --no-textconv --unified=3 --output=patch.txt",
            "git diff --no-ext-diff --no-textconv --no-index ../private ./here",
            "git rev-parse --git-dir",
            "git status --short; git add -A", "git show HEAD:.env",
            "powershell -Command Remove-Item ./file", "teamai recall forbidden",
        )
        for command in commands:
            with self.subTest(command=command):
                self.assertEqual(permission("shell", command), "ask")

    def test_dangerous_git_write_verbs_deny(self):
        commands = (
            "git reset --hard HEAD", "git restore .", "git clean -fd",
            "git checkout master", "git add -A", "git commit -am bad",
            "git push origin main", "git stash push", "git rebase master",
            "git merge feature", "git pull origin main",
        )
        for command in commands:
            with self.subTest(command=command):
                self.assertEqual(permission("shell", command), "deny")
        # A non-canonical wrapper is only ASK, never misreported as forced deny.
        self.assertEqual(permission("shell", "git -c core.hooksPath=/tmp add -A"), "ask")

    def test_business_read_glob_grep_allow_but_secrets_and_external_denied(self):
        for action, value in (
            ("read", "src/main/java/example/OrderService.java"),
            ("read", "db/migration/V2__change.SQL"),
            ("glob", "**/*.java"),
            ("grep", "Transactional"),
            ("skill", "codea-review"),
        ):
            with self.subTest(action=action, value=value):
                self.assertEqual(permission(action, value), "allow")
        self.assertEqual(permission("read", ".env"), "deny")
        self.assertEqual(permission("read", ".env.production"), "deny")
        self.assertEqual(permission("external_directory", "C:/Other/private/*"), "deny")
        self.assertEqual(permission("skill", "unrelated-skill"), "deny")

    def test_only_report_is_native_edit_exception(self):
        self.assertEqual(permission("edit", "temp/codea-review-report.md"), "allow")
        for path in (
            "src/main/java/example/OrderService.java",
            "database/V5__migration.sql", "temp/debug.txt",
            "temp/codea-review-report.md.bak", "src/temp/file.sql",
        ):
            with self.subTest(path=path):
                self.assertEqual(permission("edit", path), "deny")

    def test_skill_preserves_read_only_intent_without_claiming_enforcement(self):
        agent = AGENT.read_text(encoding="utf-8")
        skill = SKILL.read_text(encoding="utf-8")
        command = COMMAND.read_text(encoding="utf-8")
        self.assertIn("不运行构建、测试、Git 写操作", skill)
        self.assertIn("不调用 git add、stash 或 checkout", skill)
        self.assertIn("Shell", agent)
        self.assertIn("ask", agent.lower())
        self.assertIn("ask", skill.lower())
        self.assertIn("不等于强制只读保证", command)
        self.assertNotIn("git *", "\n".join(
            resource for action, resource, effect in parse_v2_permissions()
            if action == "shell" and effect == "allow"))

    def test_sql_scope_and_safety_requirements_in_skill(self):
        skill = SKILL.read_text(encoding="utf-8")
        self.assertIn(":(exclude,glob)**/.*/**", skill)
        self.assertIn(":(exclude,glob)temp/**", skill)
        self.assertIn("大小写不敏感", skill)
        self.assertIn("SQL-only", skill)
        self.assertIn("UNTRACKED", skill)
        self.assertIn("UPDATE / DELETE", skill)
        self.assertIn("DDL 兼容性", skill)
        self.assertIn("HEAD、INDEX", skill)

    def test_original_five_chapters_and_report_path(self):
        skill = SKILL.read_text(encoding="utf-8")
        template = (SKILL.parent / "references/report-template.md").read_text(encoding="utf-8")
        for title in ("评审摘要", "调用链与覆盖范围", "风险清单",
                      "问题明细", "后续处理与评审边界"):
            self.assertIn("## " + title, template)
        self.assertIn("temp/codea-review-report.md", skill)
        self.assertIn("原生 write", skill)
        self.assertIn("Git 基线", skill)
        self.assertIn("F-001", template)


class UsabilityReportPresentation(unittest.TestCase):
    """Markdown display tests only, not actual OpenCode model validation."""

    def test_report_skill_retains_review_and_save_invariants(self):
        skill = SKILL.read_text(encoding="utf-8")
        required = (
            "COMMITTED", "STAGED", "UNSTAGED", "UNTRACKED",
            ":(exclude,glob)**/.*/**", ":(exclude,glob)temp/**",
            "SQL-only", "大小写不敏感", "git merge-base",
            "幂等/重复扣减", "事务一致性", "金额精度", "权限/租户",
            "并发与状态", "缓存/DB", "边界故障",
            "temp/codea-review-report.md", "原生 write",
        )
        for clause in required:
            with self.subTest(clause=clause):
                self.assertIn(clause, skill)

    def test_five_chapter_template_groups_findings_and_keeps_metadata(self):
        template = (SKILL.parent / "references/report-template.md").read_text(encoding="utf-8")
        self.assertEqual(re.findall(r"^## (.+)$", template, re.MULTILINE), [
            "评审摘要", "调用链与覆盖范围", "风险清单",
            "问题明细", "后续处理与评审边界",
        ])
        for clause in (
            "| 📂 有效变更文件 | 🔴 HIGH | 🟡 MEDIUM |",
            "严重 N · 高 N · 中 N · 低 N",
            "| 分支 |", "| 审查状态 |", "| 变更来源 |",
            "| 风险概览 |", "#### 🔴 HIGH", "#### 🟡 MEDIUM",
            "| 编号 | 等级 | 风险类型 | 问题摘要 | 业务影响 | 建议动作 |",
            "---\n\n### 🔴 F-001", "**问题与触发条件**",
            "**业务影响**", "**代码位置**", "**关键证据**",
            "**来源：**", "**修复建议**", "**验证建议**",
        ):
            with self.subTest(clause=clause):
                self.assertIn(clause, template)

    def test_three_finding_example_is_consistent_across_list_and_details(self):
        path = ROOT / "docs/examples/codea-review-2-0-usability-report-comparison.md"
        sample = path.read_text(encoding="utf-8")
        self.assertIn("均为虚构", sample)
        self.assertIn("不提供虚构源码", sample)
        report = sample.split("# 代码评审报告｜演示项目（虚构样例）", 1)[1]
        report = report.split("## 展示回归关键点", 1)[0]
        headings = re.findall(r"^## (.+)$", report, re.MULTILINE)
        self.assertEqual(headings, [
            "评审摘要", "调用链与覆盖范围", "风险清单",
            "问题明细", "后续处理与评审边界",
        ])
        risklist = report.split("## 风险清单", 1)[1].split("## 问题明细", 1)[0]
        details = report.split("## 问题明细", 1)[1].split("## 后续处理", 1)[0]
        ids = re.findall(r"^\| (F-\d{3}) \|", risklist, re.MULTILINE)
        detail_ids = re.findall(r"^### [🔴🟡] (F-\d{3}) ", details, re.MULTILINE)
        self.assertEqual(ids, ["F-001", "F-002", "F-003"])
        self.assertEqual(detail_ids, ids)
        self.assertEqual(len(re.findall(r"^### 🔴 F-", details, re.MULTILINE)), 2)
        self.assertEqual(len(re.findall(r"^### 🟡 F-", details, re.MULTILINE)), 1)
        chunks = re.split(r"(?=^### [🔴🟡] F-)", details, flags=re.MULTILINE)[1:]
        for block in chunks:
            for label in ("**问题与触发条件**", "**业务影响**", "**代码位置**",
                          "**关键证据**", "**修复建议**", "**验证建议**"):
                with self.subTest(label=label):
                    self.assertIn(label, block)


class SeparateTemplateContracts(unittest.TestCase):
    """Static distribution/loading contracts; NOT TeamAI or TUI proof."""

    def test_skill_references_trusted_template_not_legacy_layout(self):
        skill = SKILL.read_text(encoding="utf-8")
        command = COMMAND.read_text(encoding="utf-8")
        template = (SKILL.parent / "references/report-template.md").read_text(encoding="utf-8")
        self.assertTrue(template.startswith("# 代码评审报告｜"))
        self.assertNotIn("完整复用仓库 main", skill)
        self.assertNotIn("Harness 1.8 正式 Markdown 报告格式", skill)
        self.assertNotIn("Harness 1.8", command)
        self.assertIn("## 5. Codea Review 2.0｜报告生成规范", skill)
        self.assertIn("references/report-template.md", skill)
        self.assertIn("references/report-template.md", command)
        self.assertIn("原生 `read`", skill)
        self.assertIn("REPORT_TEMPLATE_LOAD_FAILED", skill)
        self.assertIn("REPORT_TEMPLATE_LOAD_FAILED", command)
        self.assertIn("不读取业务项目同名文件", skill)
        self.assertNotIn("| 📂 有效变更文件 | 🔴 HIGH | 🟡 MEDIUM |", skill)
        self.assertEqual(re.findall(r"^## (.+)$", template, flags=re.MULTILINE), [
            "评审摘要", "调用链与覆盖范围", "风险清单",
            "问题明细", "后续处理与评审边界",
        ])

    def test_template_read_does_not_expand_agent_permissions(self):
        skill = SKILL.read_text(encoding="utf-8")
        agent = AGENT.read_text(encoding="utf-8")
        self.assertIn("不使用 Shell、PowerShell、脚本或 Git 命令绕过", skill)
        self.assertIn('action: external_directory, resource: "*", effect: deny', agent)
        self.assertNotIn('action: external_directory, resource: "*", effect: allow', agent)
        self.assertIn('action: skill, resource: codea-review, effect: allow', agent)
        self.assertIn('action: read, resource: "*", effect: allow', agent)

    def test_entire_skill_dir_distribution_and_pending_live_read(self):
        readme = (ROOT / "codea-review/README.md").read_text(encoding="utf-8")
        self.assertIn("references/report-template.md", readme)
        self.assertIn("Copy-Item -LiteralPath $from -Destination $to -Recurse", readme)
        self.assertIn("TeamAI push/pull", readme)
        self.assertIn("SKILL.md", readme)
        self.assertIn("commands/codea-review.md", readme)
        self.assertIn("PENDING", readme)

    def test_first_tool_and_old_git_review_save_contracts_preserved(self):
        skill = SKILL.read_text(encoding="utf-8")
        command = COMMAND.read_text(encoding="utf-8")
        self.assertIn("首次工具调用仍为 `skill(codea-review)`", skill)
        self.assertIn("首个工具动作必须使用 OpenCode 原生 skill", command)
        for x in ("COMMITTED", "STAGED", "UNSTAGED", "UNTRACKED", "SQL-only",
                  ":(exclude,glob)temp/**", ":(exclude,glob)**/.*/**",
                  "## 6. 保存正式报告（每次 Review 必做）",
                  "temp/codea-review-report.md"):
            with self.subTest(text=x):
                self.assertIn(x, skill)
        self.assertIn("写入失败仍", command)
        self.assertIn("模板读取失败", command)
        self.assertIn("不生成或保存正式报告", command)
        self.assertIn("模板读取成功后", command)
        self.assertIn("读取规范", command)


class CommandParameters(unittest.TestCase):
    """Execute the Skill's declarative contract, not a shipped parser/Runtime.

    This deterministic test driver cannot certify an LLM's tool use. Real
    OpenCode/private-model parsing, first-tool ordering and saving remain live QA.
    """

    def setUp(self):
        match = re.search(r"```json\n(\{\n  \"contract\": \"codea-review-arguments-v1\".*?\n\})\n```",
                          SKILL.read_text(encoding="utf-8"), re.S)
        self.assertIsNotNone(match, "Skill must define the executable argument contract")
        self.contract = json.loads(match.group(1))
        self.calls = []
        self.git_env = os.environ.copy()

    def parse(self, raw):
        c = self.contract
        if not re.fullmatch(c["input_pattern"], raw):
            raise ValueError("ARGUMENT_ERROR")
        values, positionals = {}, []
        tokens = raw.split()
        i = 0
        while i < len(tokens):
            token = tokens[i]
            if token.startswith("-"):
                if token not in c["options"] or token in values:
                    raise ValueError("ARGUMENT_ERROR")
                i += 1
                if i == len(tokens) or not re.fullmatch(c["options"][token], tokens[i]):
                    raise ValueError("ARGUMENT_ERROR")
                values[token] = tokens[i]
            else:
                positionals.append(token)
            i += 1
        if (len(positionals) > c["max_positionals"] or
                (positionals and c["positional_conflict"] in values) or
                any(not re.fullmatch(c["options"][c["positional_option"]], p)
                    for p in positionals)):
            raise ValueError("ARGUMENT_ERROR")
        base = values.get("--base")
        if base and (base in c["base_forbidden"] or
                     any(base.startswith(p) for p in c["base_forbidden_prefixes"])):
            raise ValueError("BASE_INVALID")
        return base, values.get("--controller", positionals[0] if positionals else None)

    def query(self, root, name, **values):
        argv = [part.format(**values) for part in self.contract["git"][name]]
        self.calls.append(tuple(argv))
        return subprocess.run(argv, cwd=root, capture_output=True, env=self.git_env)

    def select(self, root, raw):
        base, controller = self.parse(raw)
        c = self.contract
        for name in c["local_only_checks"]:
            result = self.query(root, name)
            if result.returncode == 0:
                raise ValueError("BASE_LOCAL_ONLY_REQUIRED")
            if result.returncode != 1:
                raise ValueError("BASE_QUERY_FAILED")
        candidates = ([(base, [prefix + base for prefix in c["ref_prefixes"]])]
                      if base is not None else
                      [(name, [ref]) for name, ref in c["default_refs"]])
        for candidate, refs in candidates:
            for ref in refs:
                if self.query(root, "check_ref", ref=ref).returncode:
                    raise ValueError("BASE_INVALID")
                result = self.query(root, "resolve", ref=ref)
                if result.returncode == 1:
                    continue
                if result.returncode:
                    raise ValueError("BASE_QUERY_FAILED")
                oid = result.stdout.decode().strip()
                if not re.fullmatch(c["oid_pattern"], oid):
                    raise ValueError("BASE_QUERY_FAILED")
                kind = self.query(root, "type", oid=oid)
                if kind.returncode:
                    raise ValueError("BASE_QUERY_FAILED")
                if kind.stdout.strip() != b"commit":
                    raise ValueError("BASE_INVALID")
                merged = self.query(root, "merge_base", oid=oid)
                if merged.returncode == 1:
                    raise ValueError("BASE_NO_MERGE_BASE")
                if merged.returncode:
                    raise ValueError("BASE_QUERY_FAILED")
                merge_base = merged.stdout.decode().strip()
                if not re.fullmatch(c["oid_pattern"], merge_base):
                    raise ValueError("BASE_QUERY_FAILED")
                return candidate, merge_base, controller
        raise ValueError("BASE_NOT_FOUND")

    def repo(self):
        temp = tempfile.TemporaryDirectory(prefix="codea params 中文 ")
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        git(root, "init", "-q", "-b", "master")
        git(root, "config", "user.name", "Parameter Test")
        git(root, "config", "user.email", "test@example.invalid")
        write(root, "db/working.SQL", "SELECT 1;\n")
        git(root, "add", ".")
        git(root, "commit", "-qm", "master baseline")
        self.master = git(root, "rev-parse", "HEAD").decode().strip()
        write(root, "db/develop.sql", "SELECT 2;\n")
        git(root, "add", ".")
        git(root, "commit", "-qm", "develop baseline")
        self.develop = git(root, "rev-parse", "HEAD").decode().strip()
        git(root, "branch", "develop")
        git(root, "update-ref", "refs/remotes/origin/develop", self.develop)
        git(root, "switch", "-qc", "feature/order")
        git(root, "update-ref", "refs/heads/master", self.master)
        write(root, "db/order.sql", "SELECT 3;\n")
        git(root, "add", ".")
        git(root, "commit", "-qm", "feature SQL")
        # Advance develop elsewhere: comparing directly with its tip is wrong.
        tree = git(root, "rev-parse", "master^{tree}").decode().strip()
        divergent = git(root, "commit-tree", tree, "-p", self.develop,
                        "-m", "divergent develop tip").decode().strip()
        git(root, "update-ref", "refs/remotes/origin/develop", divergent)
        write(root, "db/staged.sql", "SELECT 4;\n")
        git(root, "add", "db/staged.sql")
        write(root, "db/working.SQL", "SELECT 5;\n")
        write(root, "db/untracked.sql", "SELECT 6;\n")
        write(root, "temp/codea-review-report.md", "existing report\n")
        write(root, ".hidden/ignored.sql", "SELECT 7;\n")
        return root

    def snapshot(self, root):
        return (git(root, "symbolic-ref", "HEAD"), git(root, "rev-parse", "HEAD"),
                git(root, "-c", "core.fsmonitor=false", "status", "--porcelain=v1", "-z",
                    "--untracked-files=all"),
                {p.relative_to(root).as_posix(): p.read_bytes()
                 for p in root.rglob("*") if p.is_file() and ".git" not in p.relative_to(root).parts},
                (root / ".git/index").read_bytes())

    def test_T01_T02_T04_T06_compatible_forms_and_order(self):
        for raw, expected in [
            ("", (None, None)), ("OrderController", (None, "OrderController")),
            ("--controller OrderController", (None, "OrderController")),
            ("--base origin/develop", ("origin/develop", None)),
            ("--base origin/develop --controller OrderController", ("origin/develop", "OrderController")),
            ("--controller OrderController --base origin/develop", ("origin/develop", "OrderController")),
            ("OrderController --base develop", ("develop", "OrderController"))]:
            with self.subTest(raw=raw):
                self.assertEqual(self.parse(raw), expected)

    def test_T01_default_remote_precedence_and_local_fallback(self):
        root = self.repo()
        # A local branch named origin/master must not shadow the remote default.
        git(root, "branch", "origin/master", self.develop)
        self.assertEqual(self.select(root, "")[:2], ("master", self.master))
        git(root, "update-ref", "refs/remotes/origin/master", self.develop)
        self.assertEqual(self.select(root, "OrderController"),
                         ("origin/master", self.develop, "OrderController"))

    def test_T03_T05_T10_T11_real_different_bases_four_layers_sql_only(self):
        root = self.repo()
        before = self.snapshot(root)
        for raw, expected_base, expected_paths, controller in [
            ("", self.master, {"db/develop.sql", "db/order.sql"}, None),
            ("--base origin/develop", self.develop, {"db/order.sql"}, None),
            ("--base origin/develop --controller OrderController", self.develop,
             {"db/order.sql"}, "OrderController")]:
            with self.subTest(raw=raw):
                selected, merge_base, actual_controller = self.select(root, raw)
                self.assertEqual(merge_base, expected_base)
                self.assertEqual(actual_controller, controller)
                results = git_inventory(root, merge_base)
                paths = {layer: {p for _, p, _ in parsed_git_entries(layer, data)}
                         for layer, data in results.items()}
                self.assertEqual(paths, {"COMMITTED": expected_paths,
                                        "STAGED": {"db/staged.sql"},
                                        "UNSTAGED": {"db/working.SQL"},
                                        "UNTRACKED": {"db/untracked.sql"}})
                patch = git(root, "diff", "--no-ext-diff", "--no-textconv", merge_base,
                            "HEAD", *SCOPE).decode()
                self.assertIn("db/order.sql", patch)
                self.assertEqual("db/develop.sql" in patch, merge_base == self.master)
                print("PARAM_BASE_EVIDENCE", selected, merge_base, paths)
        self.assertEqual(self.snapshot(root), before)

    def test_T07_missing_ref_does_not_fallback_or_overwrite(self):
        root = self.repo()
        before = self.snapshot(root)
        with self.assertRaisesRegex(ValueError, "BASE_NOT_FOUND"):
            self.select(root, "--base origin/missing")
        self.assertFalse(any("refs/heads/master" in call for call in self.calls))
        self.assertEqual(self.snapshot(root), before)

    def test_T08_invalid_arguments_stop_before_git(self):
        for raw in ["--base", "--controller", "--base --controller OrderController",
                    "--base master --base develop", "--controller A --controller B",
                    "--unknown x", "OrderController Extra", "OrderController --controller Other",
                    "--base=master", "--controller a.b", "--controller -OrderController"]:
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                self.select(Path("unused"), raw)
        self.assertEqual(self.calls, [])

    def test_T09_injection_and_revision_rejected_without_execution(self):
        for base in ["master;touch sentinel", "master&whoami", "$(touch sentinel)",
                     "`id`", "master|id", "master>sentinel", "master<file", "master%PATH%",
                     "master!PATH!", "master\nwhoami", "master\rwhoami", "master\x00",
                     "--help", "-x", "HEAD", "HEAD~3", "A..B", ":path", "master:path",
                     "master^{commit}", "@{1}", "refs/tags/v1", "refs/heads/master",
                     "'master'", '"master"', "master\\path", "master*", "master?"]:
            with self.subTest(base=base), self.assertRaises(ValueError):
                self.select(Path("unused"), "--base " + base)
        self.assertEqual(self.calls, [])

    def test_T09_git_ref_format_and_tag_only_rejected(self):
        root = self.repo()
        git(root, "tag", "tag-only")
        for base in ["feature/../x", "feature//x", "feature/x.lock", "master.", "tag-only"]:
            with self.subTest(base=base), self.assertRaises(ValueError):
                self.select(root, "--base " + base)
        self.assertFalse(any(call[1] == "merge-base" for call in self.calls))

    def test_valid_slash_branch_and_non_commit_ref(self):
        root = self.repo()
        selected, merge_base, _ = self.select(root, "--base feature/order")
        self.assertEqual(selected, "feature/order")
        layers = git_inventory(root, merge_base)
        self.assertEqual(layers["COMMITTED"], b"")
        self.assertTrue(all(layers[layer] for layer in ("STAGED", "UNSTAGED", "UNTRACKED")))
        blob = git(root, "rev-parse", "HEAD:db/order.sql").decode().strip()
        git(root, "update-ref", "refs/remotes/origin/blob", blob)
        with self.assertRaisesRegex(ValueError, "BASE_INVALID"):
            self.select(root, "--base origin/blob")

    def test_unrelated_history_is_explicit_failure(self):
        root = self.repo()
        tree = git(root, "rev-parse", "HEAD^{tree}").decode().strip()
        orphan = git(root, "commit-tree", tree, "-m", "unrelated").decode().strip()
        git(root, "update-ref", "refs/remotes/origin/unrelated", orphan)
        with self.assertRaisesRegex(ValueError, "BASE_NO_MERGE_BASE"):
            self.select(root, "--base origin/unrelated")

    def test_default_baseline_missing_preserves_explicit_failure(self):
        root = self.repo()
        git(root, "update-ref", "-d", "refs/heads/master")
        with self.assertRaisesRegex(ValueError, "BASE_NOT_FOUND"):
            self.select(root, "")

    def test_T12_new_dynamic_queries_still_ask(self):
        root = self.repo()
        self.select(root, "--base origin/develop")
        for call in self.calls:
            with self.subTest(command=call):
                self.assertEqual(permission("shell", " ".join(call)), "ask")

    def test_promisor_missing_object_never_fetches_or_writes_git(self):
        source = self.repo()
        write(source, "db/remote-only.sql", "SELECT 999;\n")
        git(source, "add", ".")
        git(source, "commit", "-qm", "object available only at promisor remote")
        absent = git(source, "rev-parse", "HEAD").decode().strip()
        root = self.repo()
        git(root, "remote", "add", "cache", str(source))
        git(root, "config", "remote.cache.promisor", "true")
        # Deliberately dangling local tracking ref: rev-parse resolves it without
        # reading the object, whereas an unguarded cat-file implicitly fetches.
        ref = root / ".git/refs/remotes/origin/develop"
        ref.write_text(absent + "\n")
        trace = source / ".git/query-trace.log"
        self.git_env["GIT_TRACE"] = str(trace)
        before = {p.relative_to(root / ".git").as_posix(): p.read_bytes()
                  for p in (root / ".git").rglob("*") if p.is_file()}
        with self.assertRaisesRegex(ValueError, "BASE_LOCAL_ONLY_REQUIRED"):
            self.select(root, "--base origin/develop")
        after = {p.relative_to(root / ".git").as_posix(): p.read_bytes()
                 for p in (root / ".git").rglob("*") if p.is_file()}
        self.assertEqual(after, before)
        self.assertNotIn(" fetch ", trace.read_text())

    def test_partialclone_extension_and_false_promisor_conservatively_rejected(self):
        for key, value in [("extensions.partialclone", "origin"),
                           ("remote.origin.promisor", "false")]:
            with self.subTest(key=key):
                root = self.repo()
                git(root, "config", key, value)
                with self.assertRaisesRegex(ValueError, "BASE_LOCAL_ONLY_REQUIRED"):
                    self.select(root, "")
                self.assertFalse(any(call[1] in ("cat-file", "merge-base")
                                     for call in self.calls))

    def test_config_query_failure_stops_without_object_queries(self):
        root = self.repo()
        # Real Git config parsing failure, not a mocked subprocess result.
        with (root / ".git/config").open("a") as stream:
            stream.write("\n[invalid\n")
        with self.assertRaisesRegex(ValueError, "BASE_QUERY_FAILED"):
            self.select(root, "--base develop")
        self.assertEqual(len(self.calls), 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
