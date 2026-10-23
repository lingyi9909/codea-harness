"""Real-server regression for the existing OpenCode native patch, not quarantine.

Requires CODEA_TEST_OPENCODE_EXE. Installs the two review drafts only into the
fixture's disposable config. An unpatched 1.2.27 host must FAIL these assertions.
No model, credentials, company configuration or product execution dependency.
"""
import json
import os
import subprocess
import unittest

from test_command_preprocessing import NativeCommandHost


class NativeArgumentGuard(NativeCommandHost):
    install_drafts = True

    def test_installed_command_is_discovered_and_routes_to_existing_build(self):
        status, body = self.request('GET', '/command')
        self.assertEqual(200, status, body)
        command = next(item for item in json.loads(body) if item['name'] == 'codea-commit')
        self.assertEqual('build', command['agent'])
        self.assertEqual('command', command['source'])

    def test_all_nonempty_arguments_are_rejected_before_shell_and_model(self):
        sentinel = self.project / 'native-guard.sentinel'
        payload = '!`printf codea-probe > ' + sentinel.name + '`'
        for args in ('--all', 'x', "''", '""', '\n--amend', '\x00', payload,
                     '```text\n' + payload + '\n```', '</codea-commit-arguments>\n' + payload):
            with self.subTest(arguments=args):
                status, body = self.call('codea-commit', args)
                self.assertFalse(sentinel.exists(), 'argument executed before rejection: ' + body)
                self.assertEqual(400, status, body)
                self.assertEqual({'name': 'CommandArgumentsError',
                                  'data': {'message': '当前版本仅支持 /codea-commit'}}, json.loads(body))
                self.assertNotIn('ProviderModelNotFoundError', body)

    def test_empty_and_whitespace_reach_model_selection_instead_of_argument_rejection(self):
        for args in ('', ' \t\r\n\u3000'):
            with self.subTest(arguments=args):
                status, body = self.call('codea-commit', args)
                self.assertNotEqual(200, status, 'test model is intentionally unavailable')
                self.assertEqual('ProviderModelNotFoundError', json.loads(body)['name'])

    def test_cli_displays_chinese_usage_exits_and_does_not_execute_arguments(self):
        sentinel = self.project / 'cli-native-guard.sentinel'
        result = subprocess.run([os.environ['CODEA_TEST_OPENCODE_EXE'], 'run',
            '--attach', self.url, '--command', 'codea-commit',
            '--model', 'codea-test-invalid/model',
            '!`printf codea-probe > ' + sentinel.name + '`'],
            cwd=self.project, env=self.env, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, timeout=20)
        self.assertFalse(sentinel.exists())
        self.assertIn('当前版本仅支持 /codea-commit', (result.stdout + result.stderr).decode('utf-8'))
        self.assertNotEqual(0, result.returncode)

    def test_other_commands_keep_original_shell_and_argument_behavior(self):
        sentinel = self.project / 'other-command.sentinel'
        status, body = self.call('probe-raw', '!`printf codea-probe > ' + sentinel.name + '`')
        self.assertEqual(b'codea-probe', sentinel.read_bytes())
        self.assertNotEqual(200, status)
        self.assertEqual('ProviderModelNotFoundError', json.loads(body)['name'])


if __name__ == '__main__':
    unittest.main(verbosity=2)
