import copy
import importlib.util
from pathlib import Path
import unittest
import json
import subprocess
import sys
import tempfile

SCRIPT = Path(__file__).resolve().parents[1] / 'scripts' / 'prepare.py'
spec = importlib.util.spec_from_file_location('prepare', SCRIPT)
module = importlib.util.module_from_spec(spec)
if SCRIPT.exists():
    spec.loader.exec_module(module)


def fixture():
    config = {'sources': [{'id': 'orders', 'dsn': '${ORDERS_DSN}', 'query_timeout': 15, 'connection_timeout': 10},
                          {'id': 'payments', 'dsn': '${PAYMENTS_DSN}', 'query_timeout': 15, 'connection_timeout': 10}],
              'tools': [{'name': 'execute_sql', 'source': s, 'readonly': True, 'max_rows': 100} for s in ('orders', 'payments')] +
                       [{'name': 'search_objects', 'source': s} for s in ('orders', 'payments')]}
    inv = {'opencode_version': '2.0.27', 'dbhub_version': 'test-fixture', 'tools': [
        {'source': s, 'kind': k, 'action': f'dbhub_{k}_{s}'}
        for s in ('orders', 'payments') for k in ('search_objects', 'execute_sql')]}
    return config, inv


class PrepareTests(unittest.TestCase):
    def build(self, config=None, inv=None, source='orders'):
        c, i = fixture()
        self.assertTrue(hasattr(module, 'prepare'), 'prepare implementation missing')
        return module.prepare(c if config is None else config, i if inv is None else inv, source)

    def test_exact_source_tools_only(self):
        result = self.build()
        child = result['agents']['database-assistant']
        self.assertEqual(child['mode'], 'subagent')
        allows = {r['action'] for r in child['permissions'] if r['effect'] == 'allow'}
        self.assertEqual(allows, {'skill', 'dbhub_search_objects_orders', 'dbhub_execute_sql_orders'})
        self.assertEqual(child['permissions'][0], {'action': '*', 'resource': '*', 'effect': 'deny'})
        parent = result['agents']['database-entry']['permissions']
        self.assertEqual(parent, [{'action': '*', 'resource': '*', 'effect': 'deny'},
                                  {'action': 'subagent', 'resource': 'database-assistant', 'effect': 'allow'}])

    def test_v2_system_and_steps_without_legacy_fields(self):
        result = self.build()
        for name, agent in result['agents'].items():
            self.assertIn('system', agent)
            self.assertTrue(agent['system'].strip())
            self.assertEqual(agent['steps'], 8)
            self.assertFalse({'prompt', 'permission', 'bash', 'task', 'maxSteps'} & agent.keys())
            self.assertFalse({'bash', 'task'} & {r['action'] for r in agent['permissions']})
        self.assertIn('orders', result['agents']['database-assistant']['system'])
        self.assertIn('database-assistant', result['agents']['database-entry']['system'])

    def test_each_source_requires_both_toml_tools(self):
        for index in range(4):
            c, _ = fixture(); c['tools'].pop(index)
            with self.subTest(index=index), self.assertRaises(ValueError): self.build(config=c)

    def test_toml_duplicate_or_wrong_source(self):
        for index in range(4):
            c, _ = fixture(); c['tools'].append(copy.deepcopy(c['tools'][index]))
            with self.subTest(index=index), self.assertRaises(ValueError): self.build(config=c)
        for source in ('unknown', 'payments'):
            c, _ = fixture(); c['tools'][2]['source'] = source
            with self.subTest(source=source), self.assertRaises(ValueError): self.build(config=c)

    def test_search_objects_rejects_sql_and_unknown_options(self):
        for key in ('sql', 'readonly', 'max_rows'):
            c, _ = fixture(); c['tools'][2][key] = 'unexpected'
            with self.subTest(key=key), self.assertRaises(ValueError): self.build(config=c)

    def test_unknown_source(self):
        with self.assertRaises(ValueError): self.build(source='prod')

    def test_duplicate_source(self):
        c, _ = fixture(); c['sources'].append(copy.deepcopy(c['sources'][0]))
        with self.assertRaises(ValueError): self.build(config=c)

    def test_rejects_unsafe_limits(self):
        for field, values in [('readonly', [False, 'true', 1]), ('max_rows', [0, -1, 101, True])]:
            for value in values:
                with self.subTest(field=field, value=value):
                    c, _ = fixture(); c['tools'][0][field] = value
                    with self.assertRaises(ValueError): self.build(config=c)

    def test_rejects_missing_or_invalid_timeouts(self):
        for field in ('query_timeout', 'connection_timeout'):
            for value in (None, 0, -1, True, 16 if field == 'query_timeout' else 61):
                c, _ = fixture(); c['sources'][0][field] = value
                with self.subTest(field=field, value=value), self.assertRaises(ValueError): self.build(config=c)

    def test_rejects_plaintext_dsn(self):
        c, _ = fixture(); c['sources'][0]['dsn'] = 'mysql://user:example@localhost/db'
        with self.assertRaises(ValueError): self.build(config=c)

    def test_rejects_unknown_config_options(self):
        c, _ = fixture(); c['tools'][0]['sql'] = 'DELETE FROM orders'
        with self.assertRaises(ValueError): self.build(config=c)

    def test_requires_both_registered_tools(self):
        _, i = fixture(); i['tools'].pop(0)
        with self.assertRaises(ValueError): self.build(inv=i)

    def test_rejects_action_injection(self):
        for action in ('*', 'shell', 'read', 'subagent', 'skill', 'mcp*', 'x\nallow', 'edit'):
            _, i = fixture(); i['tools'][0]['action'] = action
            with self.subTest(action=action), self.assertRaises(ValueError): self.build(inv=i)

    def test_rejects_action_collision(self):
        _, i = fixture(); i['tools'][2]['action'] = i['tools'][0]['action']
        with self.assertRaises(ValueError): self.build(inv=i)

    def test_rejects_duplicate_kind_and_unknown_source(self):
        _, i = fixture(); i['tools'].append(copy.deepcopy(i['tools'][0]))
        with self.assertRaises(ValueError): self.build(inv=i)
        _, i = fixture(); i['tools'][0]['source'] = 'prod'
        with self.assertRaises(ValueError): self.build(inv=i)

    def test_cli_creates_candidate_and_refuses_overwrite(self):
        _, inv = fixture()
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            inventory = folder / 'inventory.json'
            inventory.write_text(json.dumps(inv), encoding='utf-8')
            output = folder / 'candidate.json'
            command = [sys.executable, str(SCRIPT), '--config',
                       str(SCRIPT.parents[1] / 'config/dbhub.example.toml'),
                       '--inventory', str(inventory), '--source', 'orders', '--out', str(output)]
            first = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertIn('CANDIDATE_ONLY', first.stdout)
            before = output.read_bytes()
            self.assertEqual(json.loads(before)['agents']['database-assistant']['mode'], 'subagent')
            second = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(second.returncode, 1)
            self.assertEqual(output.read_bytes(), before)

    def test_cli_invalid_inventory_writes_nothing(self):
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            inventory = folder / 'inventory.json'
            inventory.write_text('not-json-private-example', encoding='utf-8')
            output = folder / 'candidate.json'
            result = subprocess.run([sys.executable, str(SCRIPT), '--config',
                        str(SCRIPT.parents[1] / 'config/dbhub.example.toml'),
                        '--inventory', str(inventory), '--source', 'orders', '--out', str(output)],
                        capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertFalse(output.exists())
            self.assertNotIn('private-example', result.stderr)

    def test_no_mutation_or_credentials_in_output(self):
        c, i = fixture(); before = copy.deepcopy((c, i))
        result = self.build(c, i)
        self.assertEqual((c, i), before)
        self.assertNotIn('DSN', str(result))


if __name__ == '__main__': unittest.main(verbosity=2)
