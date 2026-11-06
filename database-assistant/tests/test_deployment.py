"""Synthetic evidence only: does not impersonate a live OpenCode/MCP export."""
import copy
import json
import subprocess
import tempfile
import importlib.util
from pathlib import Path
import unittest
import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from test_prepare import fixture, module as prepare_module

SCRIPT = Path(__file__).resolve().parents[1] / 'scripts/verify_deployment.py'
spec = importlib.util.spec_from_file_location('verify_deployment', SCRIPT)
module = importlib.util.module_from_spec(spec)
if SCRIPT.exists(): spec.loader.exec_module(module)


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.config, self.inventory = fixture()
        self.candidate = prepare_module.prepare(self.config, self.inventory, 'orders')
        self.observed = {'tools': [dict(t, evidence_id='synthetic-' + str(n))
                                    for n, t in enumerate(self.inventory['tools'])]}
        self.effective = [dict(agent, id=name) for name, agent in self.candidate['agents'].items()]

    def verify(self):
        self.assertTrue(hasattr(module, 'verify'), 'deployment verifier missing')
        return module.verify(self.config, self.inventory, 'orders', self.candidate, self.observed, self.effective)

    def test_matching_snapshots_are_not_runtime_pass(self):
        result = self.verify()
        self.assertEqual(result['status'], 'SNAPSHOT_MATCH_ONLY')
        self.assertEqual(result['runtime_acceptance'], 'PENDING')

    def test_swapped_source_tool_names_rejected(self):
        original = copy.deepcopy(self.inventory)
        for left, right in ((0, 2), (1, 3)):
            self.inventory = copy.deepcopy(original)
            self.inventory['tools'][left]['action'], self.inventory['tools'][right]['action'] = (
                self.inventory['tools'][right]['action'], self.inventory['tools'][left]['action'])
            self.candidate = prepare_module.prepare(self.config, self.inventory, 'orders')
            self.effective = [dict(a, id=n) for n, a in self.candidate['agents'].items()]
            with self.subTest(tool=left), self.assertRaises(ValueError): self.verify()

    def test_missing_duplicate_extra_registration_rejected(self):
        original = copy.deepcopy(self.observed)
        for mutation in ('missing', 'duplicate', 'extra'):
            self.observed = copy.deepcopy(original)
            if mutation == 'missing': self.observed['tools'].pop()
            elif mutation == 'duplicate': self.observed['tools'].append(self.observed['tools'][0])
            else: self.observed['tools'].append(dict(self.observed['tools'][0], action='another_execute_sql'))
            with self.subTest(mutation=mutation), self.assertRaises(ValueError): self.verify()

    def test_parent_bypass_rule_rejected(self):
        self.effective[1]['permissions'].append({'action': 'dbhub_execute_sql_orders', 'resource': '*', 'effect': 'allow'})
        with self.assertRaises(ValueError): self.verify()

    def test_candidate_tampering_rejected(self):
        self.candidate['agents']['database-assistant']['system'] = 'untrusted'
        with self.assertRaises(ValueError): self.verify()

    def test_missing_system_or_steps_rejected(self):
        for field in ('system', 'steps'):
            original = copy.deepcopy(self.effective)
            del self.effective[0][field]
            with self.subTest(field=field), self.assertRaises(ValueError): self.verify()
            self.effective = original

    def test_cli_checks_utf8_bom_snapshots_without_executing_tools(self):
        with tempfile.TemporaryDirectory() as tmp:
            args = [sys.executable, str(SCRIPT), '--source', 'orders', '--config',
                    str(SCRIPT.parents[1] / 'config/dbhub.example.toml')]
            for key, value in [('inventory', self.inventory), ('candidate', self.candidate),
                               ('observed', self.observed), ('effective', self.effective)]:
                path = Path(tmp) / (key + '.json')
                path.write_text(json.dumps(value), encoding='utf-8-sig')
                args.extend(['--' + key, str(path)])
            result = subprocess.run(args, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)['runtime_acceptance'], 'PENDING')
            (Path(tmp) / 'observed.json').write_text('{}')
            result = subprocess.run(args, capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertIn('DEPLOYMENT_MISMATCH', result.stderr)

    def test_other_agent_and_missing_evidence_rejected(self):
        self.effective.append(dict(self.effective[0], id='build'))
        with self.assertRaises(ValueError): self.verify()
        self.effective.pop()
        self.observed['tools'][0]['evidence_id'] = ''
        with self.assertRaises(ValueError): self.verify()


if __name__ == '__main__': unittest.main(verbosity=2)
