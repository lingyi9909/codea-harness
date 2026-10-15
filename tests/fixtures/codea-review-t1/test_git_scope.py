"""Four-layer Git scope and hidden-directory exclusion regression.

Run: python tests/fixtures/codea-review-t1/test_git_scope.py
Creates only a disposable temporary Git repository; no business files touched.
"""
from pathlib import Path
import subprocess
import tempfile
import unittest

EXCLUDE = ":(exclude,glob)**/.*/**"


def git(root, *args):
    return subprocess.run(["git", "-C", str(root), *args], capture_output=True, check=True).stdout


def names(raw):
    fields = [field.decode("utf-8") for field in raw.split(b"\0") if field]
    result = set()
    index = 0
    while index < len(fields):
        status = fields[index]
        index += 1
        if status[0] in "RC":
            result.update(fields[index:index + 2])
            index += 2
        else:
            result.add(fields[index])
            index += 1
    return result


class GitScopeTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="codea 中文 空格 ")
        self.root = Path(self.tmp.name)
        git(self.root, "init", "-q", "-b", "master")
        git(self.root, "config", "user.name", "Codea")
        git(self.root, "config", "user.email", "codea@example.invalid")
        for p in ("src/main/Committed.java", "src/main/Staged.java", "src/main/Working.java",
                  ".github/actions.yml", "src/.teamai/config.java", "sub/.claude/secret.java"):
            self.write(p, "before\n")
        git(self.root, "add", "-A")
        git(self.root, "commit", "-qm", "baseline")
        git(self.root, "checkout", "-q", "-b", "feature")
        self.write("src/main/Committed.java", "committed\n")
        self.write(".github/actions.yml", "hidden committed\n")
        git(self.root, "add", "-A")
        git(self.root, "commit", "-qm", "feature")
        self.write("src/main/Staged.java", "staged\n")
        git(self.root, "add", "-A")
        self.write("src/main/Working.java", "unstaged\n")
        self.write("src/.teamai/config.java", "hidden unstaged\n")
        self.write("src/main/新增.java", "untracked\n")
        self.write("src/.agents/new.java", "hidden untracked\n")

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, p, value):
        dest = self.root / p
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(value, encoding="utf-8")

    def test_all_four_layers_filter_hidden_directories_in_git(self):
        opts = ("--no-ext-diff", "--no-textconv", "--name-status", "-z")
        scope = ("--", ".", EXCLUDE)
        committed = names(git(self.root, "diff", *opts, "master...HEAD", *scope))
        staged = names(git(self.root, "diff", *opts, "--cached", "HEAD", *scope))
        unstaged = names(git(self.root, "diff", *opts, *scope))
        untracked = set(filter(None, git(self.root, "ls-files", "--others",
                               "--exclude-standard", "-z", *scope).decode("utf-8").split("\0")))
        self.assertEqual(committed, {"src/main/Committed.java"})
        self.assertEqual(staged, {"src/main/Staged.java"})
        self.assertEqual(unstaged, {"src/main/Working.java"})
        self.assertEqual(untracked, {"src/main/新增.java"})
        self.assertEqual(len(committed | staged | unstaged | untracked), 4)

    def test_dirty_worktree_has_separate_index_and_head_evidence(self):
        status = git(self.root, "-c", "core.fsmonitor=false", "status",
                     "--porcelain=v1", "-z", "--untracked-files=all")
        self.assertIn(b"Staged.java", status)
        self.assertIn(b"Working.java", status)
        self.assertIn("新增.java".encode(), status)
        self.assertEqual(git(self.root, "show", "HEAD:src/main/Staged.java"), b"before\n")
        self.assertEqual(git(self.root, "show", ":src/main/Staged.java"), b"staged\n")

    def test_nested_hidden_directory_is_excluded(self):
        result = git(self.root, "diff", "--name-only", "-z", "master...HEAD",
                     "--", ".", EXCLUDE).decode("utf-8")
        self.assertNotIn(".github", result)
        self.assertIn("src/main/Committed.java", result)


if __name__ == "__main__":
    unittest.main(verbosity=2)
