"""Tests of the fixture oracle, never a substitute for OpenCode acceptance."""
import importlib.util
import sys
import pathlib
import os
import subprocess
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent

def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value

class FixtureOracle(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='codea fixture 中文 ')
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.setup = module('setup')
        self.verify = module('verify')

    def git(self, repo, *args, check=True):
        return subprocess.run(['git', '-C', str(repo), *args], check=check,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def create(self, case):
        repo, manifest = self.setup.create(case, self.root / case)
        return repo, manifest

    def commit(self, repo, message='fix(fixture): 保留暂存边界\n'):
        msg = self.root / '消息.txt'
        msg.write_text(message, encoding='utf-8')
        return self.git(repo, 'commit', '--cleanup=verbatim', '-F', str(msg), check=False)

    def checkpoint(self, repo, expectation, message=None):
        manifest = self.root / ('checkpoint-' + str(len(list(self.root.glob('checkpoint-*')))) + '.json')
        command = [sys.executable, str(HERE / 'checkpoint.py'), str(repo), str(manifest),
                   '--expect', expectation]
        if message is not None:
            msg = self.root / 'confirmed-message.txt'
            msg.write_text(message, encoding='utf-8')
            command += ['--message-file', str(msg)]
        result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertEqual(0, result.returncode, result.stderr.decode())
        return manifest

    def test_checkpoint_subdirectory_cannot_write_inside_actual_worktree(self):
        repo, _ = self.create('C02')
        subdir = repo / 'nested'
        subdir.mkdir()
        bad = repo / 'should-not-exist.json'
        result = subprocess.run([sys.executable, str(HERE / 'checkpoint.py'),
                                 str(subdir), str(bad), '--expect', 'unchanged'],
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertNotEqual(0, result.returncode)
        self.assertFalse(bad.exists())
        good = self.root / 'external-checkpoint.json'
        result = subprocess.run([sys.executable, str(HERE / 'checkpoint.py'),
                                 str(subdir), str(good), '--expect', 'unchanged'],
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertEqual(0, result.returncode, result.stderr.decode())
        self.assertEqual([], self.verify.verify(repo, good))

    def test_pending_or_cancelled_preview_rejects_any_commit(self):
        repo, _ = self.create('C02')
        manifest = self.checkpoint(repo, 'unchanged')
        self.assertEqual([], self.verify.verify(repo, manifest))
        self.commit(repo)
        self.assertIn('head changed', '\n'.join(self.verify.verify(repo, manifest)))

    def test_cancel_preserves_newly_prepared_staging(self):
        repo, _ = self.create('C03')
        self.git(repo, '--literal-pathspecs', 'add', '--', 'task.txt', 'new.txt', 'delete.txt')
        manifest = self.checkpoint(repo, 'unchanged')
        self.assertEqual([], self.verify.verify(repo, manifest))
        self.git(repo, 'reset', '--', 'task.txt')  # Deliberately wrong cancellation control.
        self.assertIn('index changed', '\n'.join(self.verify.verify(repo, manifest)))

    def test_confirmed_checkpoint_checks_full_message_and_partial_index(self):
        repo, _ = self.create('C02')
        message = 'fix(fixture): 只提交暂存内容\n\n保留工作区 B 的变化。\n'
        manifest = self.checkpoint(repo, 'commit', message)
        self.assertIn('expected one new commit', '\n'.join(self.verify.verify(repo, manifest)))
        self.git(repo, 'commit', '--cleanup=verbatim', '-F', str(self.root / 'confirmed-message.txt'))
        self.assertEqual([], self.verify.verify(repo, manifest))

    def test_confirmed_text_survives_git_cleanup_and_comment_settings(self):
        message = ('fix(fixture): 展示并确认说明\n\n'
                   '# 保留的业务说明\n; 自定义注释字符也是正文\n正文尾部有空格  \n')
        for name, cleanup, comment in [('default', None, None), ('strip', 'strip', None),
                                       ('custom-comment', 'strip', ';'),
                                       ('whitespace', 'whitespace', ';')]:
            with self.subTest(cleanup=name):
                repo, _ = self.setup.create('C01', self.root / name)
                if cleanup:
                    self.git(repo, 'config', '--local', 'commit.cleanup', cleanup)
                if comment:
                    self.git(repo, 'config', '--local', 'core.commentChar', comment)
                manifest = self.checkpoint(repo, 'commit', message)
                self.assertEqual(0, self.commit(repo, message).returncode)
                self.assertEqual([], self.verify.verify(repo, manifest))
                # Assert the raw commit body too, independently of the oracle.
                raw = self.git(repo, 'cat-file', 'commit', 'HEAD').stdout.split(b'\n\n', 1)[1]
                self.assertEqual(message.encode('utf-8'), raw)

    def test_implicit_cleanup_negative_control_changes_confirmed_message(self):
        repo, _ = self.create('C01')
        self.git(repo, 'config', '--local', 'commit.cleanup', 'strip')
        message = 'fix(fixture): 清理负对照\n\n# 必须保留的正文\n行尾空白  \n'
        manifest = self.checkpoint(repo, 'commit', message)
        self.git(repo, 'commit', '-F', str(self.root / 'confirmed-message.txt'))
        self.assertIn('message differs', '\n'.join(self.verify.verify(repo, manifest)))
        raw = self.git(repo, 'cat-file', 'commit', 'HEAD').stdout.split(b'\n\n', 1)[1]
        self.assertEqual('fix(fixture): 清理负对照\n\n行尾空白\n'.encode('utf-8'), raw)

    def test_unconfirmed_message_change_is_detected(self):
        repo, _ = self.create('C01')
        manifest = self.checkpoint(repo, 'commit', 'fix(fixture): 用户确认的说明\n')
        self.commit(repo)  # Deliberately commits a different title.
        self.assertIn('message differs', '\n'.join(self.verify.verify(repo, manifest)))

    def test_preview_checkpoint_detects_stale_index_head_or_branch(self):
        for change in ('index', 'head', 'branch'):
            with self.subTest(change=change):
                repo, _ = self.setup.create('C01', self.root / change)
                manifest = self.checkpoint(repo, 'unchanged')
                if change == 'index':
                    (repo / 'task.txt').write_text('outside change\n')
                    self.git(repo, 'add', '--', 'task.txt')
                elif change == 'head':
                    self.commit(repo)
                else:
                    self.git(repo, 'switch', '-c', 'outside-branch')
                self.assertTrue(self.verify.verify(repo, manifest))

    def test_partial_staging_requires_commit_then_accepts_only_A(self):
        repo, manifest = self.create('C02')
        self.assertTrue(any('HEAD' in e for e in self.verify.verify(repo, manifest)))
        self.assertEqual(0, self.commit(repo).returncode)
        self.assertEqual([], self.verify.verify(repo, manifest))
        self.assertEqual(b'A=1\n' + b'padding\n' * 12 + b'B=0\n',
                         self.git(repo, 'show', 'HEAD:shared.txt').stdout)
        self.assertIn(b'B=1', (repo / 'shared.txt').read_bytes())
        self.assertEqual(b'unrelated\n', (repo / 'other.txt').read_bytes())

    def test_expanding_partial_staging_detects_B_and_unrelated_file(self):
        repo, manifest = self.create('C02')
        self.git(repo, 'add', '--', 'shared.txt', 'other.txt')
        self.commit(repo)
        errors = '\n'.join(self.verify.verify(repo, manifest))
        self.assertIn('commit tree: shared.txt', errors)
        self.assertIn('commit tree: other.txt', errors)

    def test_worktree_and_index_tampering_are_detected(self):
        repo, manifest = self.create('C04')
        self.assertEqual([], self.verify.verify(repo, manifest))
        (repo / 'task.txt').write_bytes(b'tampered\n')
        self.git(repo, 'add', '--', 'task.txt')
        errors = '\n'.join(self.verify.verify(repo, manifest))
        self.assertIn('index', errors)
        self.assertIn('worktree: task.txt', errors)

    def test_nonempty_destination_is_never_overwritten(self):
        dest = self.root / 'occupied'
        dest.mkdir()
        (dest / 'keep.txt').write_bytes(b'keep')
        with self.assertRaises(ValueError):
            self.setup.create('C01', dest)
        self.assertEqual(b'keep', (dest / 'keep.txt').read_bytes())

    def test_normal_commit_controls(self):
        for case in ('C01', 'C03', 'C07', 'C08', 'C14', 'C15-binary', 'C15-worktree'):
            with self.subTest(case=case):
                repo, manifest = self.create(case)
                if case == 'C03':
                    self.git(repo, '--literal-pathspecs', 'add', '--', 'task.txt', 'new.txt', 'delete.txt')
                if case == 'C14':
                    self.git(repo, '--literal-pathspecs', 'add', '--', '.gitignore', '.github/config.yml')
                self.assertEqual(0, self.commit(repo).returncode)
                self.assertEqual([], self.verify.verify(repo, manifest))

    def test_stop_controls_preserve_state(self):
        for case in self.setup.CASES:
            if self.setup.CASES[case] != 'stop':
                continue
            with self.subTest(case=case):
                repo, manifest = self.create(case)
                self.assertEqual([], self.verify.verify(repo, manifest))

    def test_hook_failure_and_rewrites_are_observed(self):
        for case in ('C09-hook', 'C09-identity', 'C09-sign', 'C09-lock',
                     'C10-message', 'C10-index', 'C10-post'):
            with self.subTest(case=case):
                repo, manifest = self.create(case)
                result = self.commit(repo)
                if case.startswith('C09'):
                    self.assertNotEqual(0, result.returncode)
                    self.assertEqual([], self.verify.verify(repo, manifest))
                elif case == 'C10-index':
                    self.assertIn('commit tree: task.txt', '\n'.join(self.verify.verify(repo, manifest)))
                else:
                    self.assertEqual([], self.verify.verify(repo, manifest))
                    if case == 'C10-message':
                        self.assertIn('hook 改写消息', self.git(repo, 'log', '-1', '--format=%B').stdout.decode('utf-8'))
                    else:
                        # A post-commit hook returning nonzero usually does NOT fail git commit.
                        self.assertEqual(b'post-hook\n', (repo / 'post.txt').read_bytes())

    @unittest.skipIf(os.name == 'nt', 'POSIX executable mode only')
    def test_worktree_executable_mode_change_is_detected(self):
        repo, manifest = self.create('C04')
        path = repo / 'task.txt'
        path.chmod(path.stat().st_mode | 0o111)
        self.assertIn('worktree: task.txt', '\n'.join(self.verify.verify(repo, manifest)))

    @unittest.skipIf(os.name == 'nt', 'directory symlink requires separate Windows privilege acceptance')
    def test_added_directory_symlink_is_detected_without_following_it(self):
        repo, manifest = self.create('C04')
        external = self.root / 'external'
        external.mkdir()
        (external / 'never-read.txt').write_bytes(b'external')
        (repo / 'extra-directory').symlink_to(external, target_is_directory=True)
        errors = '\n'.join(self.verify.verify(repo, manifest))
        self.assertIn('worktree: extra-directory', errors)
        self.assertNotIn('never-read.txt', errors)

    def test_configuration_and_hooks_must_not_be_bypassed_or_replaced(self):
        repo, manifest = self.create('C09-hook')
        self.git(repo, 'config', '--local', 'core.hooksPath', str(repo / '.git' / 'empty-hooks'))
        self.assertIn('protection', '\n'.join(self.verify.verify(repo, manifest)))

    def test_second_commit_or_branch_switch_is_rejected(self):
        repo, manifest = self.create('C01')
        self.commit(repo)
        (repo / 'next.txt').write_bytes(b'next\n')
        self.git(repo, 'add', '--', 'next.txt')
        self.commit(repo)
        self.assertIn('parents', '\n'.join(self.verify.verify(repo, manifest)))
        self.git(repo, 'switch', '-c', 'wrong')
        self.assertIn('branch', '\n'.join(self.verify.verify(repo, manifest)))

if __name__ == '__main__':
    unittest.main(verbosity=2)
