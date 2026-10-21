"""Optional real-host security probe; requires an explicitly supplied test binary.

Never installs OpenCode, calls a model, or copies files to a real user config.
With an active Command on a vulnerable host this suite MUST fail.
With the entry withdrawn it tests quarantine, not a functional fix.
"""
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import unittest
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
BINARY = os.environ.get('CODEA_TEST_OPENCODE_EXE')


@unittest.skipUnless(BINARY, 'set CODEA_TEST_OPENCODE_EXE to an approved test binary')
class NativeCommandBoundary(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='codea-command-boundary-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.base = Path(cls.temp.name)
        cls.project = cls.base / 'project'
        cls.project.mkdir()
        commands = cls.project / '.opencode' / 'commands'
        commands.mkdir(parents=True)
        source = ROOT / 'codea-commit' / 'commands'
        if source.exists():
            for path in source.glob('*.md'):
                shutil.copyfile(path, commands / path.name)
        skill_source = ROOT / 'codea-commit' / 'skills'
        if skill_source.exists():
            shutil.copytree(skill_source, cls.project / '.opencode' / 'skills')
        probe_skill = cls.project / '.opencode' / 'skills' / 'probe-skill' / 'SKILL.md'
        probe_skill.parent.mkdir(parents=True, exist_ok=True)
        probe_skill.write_text('---\nname: probe-skill\ndescription: Disposable security control\n---\nReject all arguments.\n', encoding='utf-8')
        # Handwritten negative controls run only in this disposable project.
        (commands / 'probe-raw.md').write_text('Reject all arguments: $ARGUMENTS\n', encoding='utf-8')
        (commands / 'probe-fallback.md').write_text('Reject all arguments.\n', encoding='utf-8')
        (commands / 'probe-fence.md').write_text('Reject all arguments:\n```text\n$ARGUMENTS\n```\n', encoding='utf-8')
        env = {k: v for k, v in os.environ.items() if k in (
            'PATH', 'HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'TMPDIR',
            'SYSTEMROOT', 'SystemRoot', 'WINDIR', 'COMSPEC', 'PATHEXT', 'TEMP', 'TMP')}
        for name in ('CONFIG', 'DATA', 'CACHE', 'STATE'):
            target = cls.base / name.lower()
            target.mkdir()
            env['XDG_' + name + '_HOME'] = str(target)
        env['OPENCODE_CONFIG_DIR'] = str(cls.base / 'config')
        env['OPENCODE_CONFIG_CONTENT'] = json.dumps({
            'model': 'codea-test-invalid/model', 'permission': 'deny',
            'plugin': [], 'autoupdate': False})
        for suffix in ('AUTOUPDATE', 'DEFAULT_PLUGINS', 'MODELS_FETCH', 'LSP_DOWNLOAD',
                       'CLAUDE_CODE', 'EXTERNAL_SKILLS'):
            env['OPENCODE_DISABLE_' + suffix] = 'true'
        env['OPENCODE_EXPERIMENTAL_DISABLE_FILEWATCHER'] = 'true'
        cls.version = subprocess.check_output([BINARY, '--version'], env=env,
                                              cwd=cls.project, timeout=30).decode().strip()
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        cls.url = 'http://127.0.0.1:{}'.format(port)
        log = (cls.base / 'server.log').open('wb')
        cls.addClassCleanup(log.close)
        cls.server = subprocess.Popen([BINARY, 'serve', '--hostname', '127.0.0.1', '--port', str(port)],
                                      cwd=cls.project, env=env, stdout=log, stderr=log)
        cls.addClassCleanup(cls.stop)
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            if cls.server.poll() is not None:
                raise RuntimeError((cls.base / 'server.log').read_text(errors='replace'))
            try:
                cls.request('GET', '/global/health')
                break
            except (OSError, urllib.error.URLError):
                time.sleep(0.1)
        else:
            raise RuntimeError('isolated OpenCode did not start')
        print('NATIVE_HOST_VERSION', cls.version, flush=True)

    @classmethod
    def stop(cls):
        if cls.server.poll() is None:
            cls.server.terminate()
            try:
                cls.server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                cls.server.kill()
                cls.server.wait(timeout=5)

    @classmethod
    def request(cls, method, path, body=None):
        data = json.dumps(body).encode() if body is not None else None
        request = urllib.request.Request(cls.url + path, data=data, method=method,
                                         headers={'Content-Type': 'application/json'})
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                return response.status, response.read().decode('utf-8')
        except urllib.error.HTTPError as error:
            return error.code, error.read().decode('utf-8')

    def call(self, command, arguments):
        status, body = self.request('POST', '/session', {})
        self.assertEqual(200, status, body)
        session = json.loads(body)['id']
        return self.request('POST', '/session/' + session + '/command',
                            {'command': command, 'arguments': arguments,
                             'model': 'codea-test-invalid/model', 'agent': 'build'})

    def test_raw_fallback_and_fence_controls_prove_model_guard_is_too_late(self):
        for command in ('probe-raw', 'probe-fallback', 'probe-fence', 'probe-skill'):
            with self.subTest(command=command):
                sentinel = self.project / (command + '.sentinel')
                status, body = self.call(command, '!`printf codea-probe > ' + sentinel.name + '`')
                self.assertTrue(sentinel.exists(), 'control did not reproduce on this host: ' + body)
                self.assertEqual(b'codea-probe', sentinel.read_bytes())
                self.assertNotEqual(200, status, 'invalid model must prevent model execution')
                self.assertIn('codea-test-invalid', body)
                print('VULNERABLE_CONTROL', command, 'sentinel_written_before_model_error', flush=True)

    def test_codea_entry_does_not_execute_shell_from_nonempty_arguments(self):
        sentinel = self.project / 'codea-entry.sentinel'
        status, body = self.call('codea-commit', '!`printf codea-probe > ' + sentinel.name + '`')
        self.assertFalse(sentinel.exists(), 'argument shell executed before model rejection: ' + body)
        self.assertNotEqual(200, status)
        self.assertNotIn('ProviderModelNotFoundError', body, 'withdrawn command must not reach model lookup')

    def test_unverified_entry_is_not_discovered(self):
        status, body = self.request('GET', '/command')
        self.assertEqual(200, status, body)
        names = {item['name'] for item in json.loads(body)}
        self.assertNotIn('codea-commit', names, 'unsafe entry must stay withdrawn until native guard is verified')


if __name__ == '__main__':
    unittest.main(verbosity=2)
