"""Codea Review 2.0 hardening: deterministic Git, permissions, and report contracts.

No model run here: real Windows OpenCode + private model Finding accuracy is separate.
"""
from __future__ import annotations

import ast
import fnmatch
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
        for layer in ("COMMITTED", "STAGED", "UNSTAGED"):
            self.assertIn(self.shared.encode() + b"\0", results[layer])
        names = {self.shared, self.committed, self.staged, self.unstaged, self.untracked}
        self.assertEqual(len(names), 5)
        self.assertEqual(sum(self.shared.encode() + b"\0" in data for data in results.values()), 3)

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
    def test_original_entry_and_v2_format(self):
        agent = AGENT.read_text(encoding="utf-8")
        command = COMMAND.read_text(encoding="utf-8")
        self.assertIn("\npermissions:\n", agent)
        self.assertNotIn("\npermission:\n", agent)
        self.assertIn("agent: codea-reviewer", command)
        self.assertIn("codea-review", command)
        self.assertNotIn("teamai", agent.lower())
        self.assertNotIn("teamai", command.lower())

    def test_git_read_queries_match_real_skill_parameters(self):
        scope = "-- . ':(exclude,glob)**/.*/**' ':(exclude,glob)temp/**'"
        queries = [
            "git --version", "git rev-parse --show-toplevel",
            "git rev-parse HEAD", "git rev-parse --abbrev-ref HEAD",
            "git rev-parse origin/master", "git rev-parse master",
            "git -c core.fsmonitor=false status --porcelain=v1 -z --untracked-files=all",
            "git merge-base master HEAD",
            f"git diff --no-ext-diff --no-textconv --name-status -z abc HEAD {scope}",
            f"git diff --no-ext-diff --no-textconv --name-status -z --cached HEAD {scope}",
            f"git diff --no-ext-diff --no-textconv --name-status -z {scope}",
            f"git ls-files --others --exclude-standard -z {scope}",
            "git diff --no-ext-diff --no-textconv --unified=3 abc HEAD -- 'db/migration/V1__one.SQL'",
            "git diff --no-ext-diff --no-textconv --unified=3 --cached HEAD -- 'db/migration/V1__one.SQL'",
            "git diff --no-ext-diff --no-textconv --unified=3 -- 'db/migration/V1__one.SQL'",
            "git show --no-ext-diff --no-textconv HEAD:db/migration/V1__one.SQL",
            "git show --no-ext-diff --no-textconv :db/migration/V1__one.SQL",
        ]
        for q in queries:
            with self.subTest(query=q):
                self.assertEqual(permission("shell", q), "allow")

    def test_writes_and_shell_composition_stay_denied(self):
        rejected = [
            "git add -A", "git commit -am x", "git reset --hard HEAD",
            "git clean -fd", "git checkout master", "git fetch origin",
            "git push origin master", "git diff --output=/tmp/exfil",
            "git diff --no-ext-diff --no-textconv --unified=3 --output=x HEAD",
            "git diff --no-ext-diff --no-textconv --unified=3 --ext-diff HEAD",
            "git diff --no-ext-diff --no-textconv --unified=3 --textconv HEAD",
            "git diff --no-ext-diff --no-textconv --unified=3 HEAD; git commit -am x",
            "git show --no-ext-diff --no-textconv HEAD:db.sql | powershell",
            "git merge-base HEAD && git reset --hard HEAD",
            "git show --no-ext-diff --no-textconv $(echo foo)",
            "git diff --no-ext-diff --no-textconv --unified=3 %TEMP%",
            "teamai recall x",
        ]
        for q in rejected:
            with self.subTest(query=q):
                self.assertEqual(permission("shell", q), "deny")
        self.assertEqual(permission("edit", "src/main/java/OrderService.java"), "deny")
        self.assertEqual(permission("edit", "database/V5__migration.SQL"), "deny")
        self.assertEqual(permission("edit", "temp/codea-review-report.md"), "allow")
        self.assertEqual(permission("external_directory", "/other/private"), "deny")

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
        for title in ("评审摘要", "调用链与覆盖范围", "风险清单",
                      "问题明细", "后续处理与评审边界"):
            self.assertIn("## " + title, skill)
        self.assertIn("temp/codea-review-report.md", skill)
        self.assertIn("原生 write", skill)
        self.assertIn("Git 基线", skill)
        self.assertIn("F-001", skill)


if __name__ == "__main__":
    unittest.main(verbosity=2)
