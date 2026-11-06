import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[1]

def load(name):
    path = ROOT / 'acceptance' / (name + '.py')
    spec = importlib.util.spec_from_file_location(name, path)
    mod = importlib.util.module_from_spec(spec)
    if path.exists(): spec.loader.exec_module(mod)
    return mod

class AcceptanceTests(unittest.TestCase):
    def test_permission_denial_requires_specific_code(self):
        mod = load('mysql_checks')
        self.assertTrue(hasattr(mod, 'assess'), 'acceptance checker missing')
        case = {'expect': 'denied'}
        self.assertTrue(mod.assess(case, 1, '', 'ERROR 1142 (42000): denied'))
        for code in (0, 1045, 1064, 2003, 2013):
            self.assertFalse(mod.assess(case, 1 if code else 0, '', f'ERROR {code}: rejected'))

    def test_query_results_exact_not_only_exit_code(self):
        mod = load('mysql_checks')
        self.assertTrue(hasattr(mod, 'assess'), 'acceptance checker missing')
        case = {'expect': 'rows', 'rows': [['10001', '19.90']]}
        self.assertTrue(mod.assess(case, 0, '10001\t19.90\n', ''))
        self.assertFalse(mod.assess(case, 0, '10001\t1990\n', ''))
        self.assertFalse(mod.assess(case, 1, '10001\t19.90\n', ''))
        self.assertTrue(mod.assess({'expect': 'rows', 'rows': []}, 0, '', ''))

    def test_bad_login_name_never_becomes_option(self):
        mod = load('mysql_checks')
        self.assertTrue(hasattr(mod, 'command'), 'acceptance checker missing')
        for value in ('--password=secret', 'a b', 'a;whoami'):
            with self.assertRaises(ValueError): mod.command('mysql', value, 'SELECT 1')
        argv = mod.command('mysql', 'test_ro', 'SELECT 1')
        self.assertIn('--login-path=test_ro', argv)
        self.assertIn('SELECT 1', argv)

    def test_stage_refuses_existing_directory_and_preserves_configs(self):
        mod = load('stage')
        self.assertTrue(hasattr(mod, 'stage'), 'stage implementation missing')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); official = root / 'dbhub'; official.mkdir()
            (official / 'SKILL.md').write_text('---\nname: dbhub\n---\nOfficial fixture')
            base = {'permissions': [{'action':'*','resource':'*','effect':'deny'}], 'mcp': {'test': {}}}
            candidate = json.loads((ROOT / 'evidence/generated.synthetic.json').read_text())
            target = root / 'new'
            mod.stage(target, base, candidate, official)
            self.assertEqual(json.loads((target / 'opencode.json').read_text())['agents'], candidate['agents'])
            self.assertTrue((target / '.opencode/skills/database-assistant/SKILL.md').exists())
            self.assertTrue((target / '.opencode/skills/dbhub/SKILL.md').exists())
            self.assertFalse((target / '.opencode/agents').exists())
            before = (target / 'opencode.json').read_bytes()
            with self.assertRaises((ValueError, FileExistsError)): mod.stage(target, base, candidate, official)
            self.assertEqual((target / 'opencode.json').read_bytes(), before)
            self.assertNotIn('agents', base)

    def test_stage_rejects_broad_default_and_legacy_candidate(self):
        mod = load('stage')
        self.assertTrue(hasattr(mod, 'stage'), 'stage implementation missing')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); official = root / 'dbhub'; official.mkdir()
            (official / 'SKILL.md').write_text('---\nname: dbhub\n---\nfixture')
            candidate = json.loads((ROOT / 'evidence/generated.synthetic.json').read_text())
            with self.assertRaises(ValueError): mod.stage(root/'bad', {}, candidate, official)
            candidate['agents']['database-assistant']['prompt'] = 'legacy'
            with self.assertRaises(ValueError): mod.stage(root/'bad2', {'permissions':[{'action':'*','resource':'*','effect':'deny'}]}, candidate, official)
            self.assertFalse((root/'bad').exists())
            self.assertFalse((root/'bad2').exists())

    def test_canary_failure_prevents_attack_queries(self):
        mod = load('mysql_checks')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); executable = root/'mysql.exe'; executable.write_text('fixture')
            out = root/'run'
            argv = ['mysql_checks', '--mysql', str(executable), '--orders-login', 'orders',
                    '--payments-login', 'payments', '--out', str(out), '--confirm-isolated']
            with patch.object(mod.sys, 'argv', argv), patch.dict(mod.os.environ, {}, clear=True), \
                 patch.object(mod, 'run', return_value=SimpleNamespace(returncode=0, stdout='WRONG', stderr='')) as runner:
                self.assertEqual(mod.main(), 1)
                self.assertEqual(runner.call_count, 1)
            report=json.loads((out/'mysql-direct.json').read_text())
            self.assertEqual(report['checks'], [])
            self.assertEqual(report['stage_acceptance'], 'PENDING')

    def test_unexpected_write_success_stops_without_saving_output(self):
        mod = load('mysql_checks')
        calls=[]
        cases=json.loads((ROOT/'acceptance/cases.json').read_text())
        def backend(mysql, login, sql):
            calls.append(sql)
            if 'v_canary' in sql:
                text='DBASSIST_T1_SYNTHETIC_' + ('ORDERS' if 'orders' in sql else 'PAYMENTS') + '_V1'
            elif sql == 'SELECT VERSION()': text='8.0.test'
            elif sql.startswith('SELECT id,value'): text='1\t123'
            elif sql.startswith('UPDATE '): text='UNEXPECTED_PRIVATE_OUTPUT'
            else:
                case=next(c for c in cases if c['sql']==sql)
                if case['expect']=='error': return SimpleNamespace(returncode=1,stdout='',stderr='ERROR 1054: test')
                text='\n'.join('\t'.join(row) for row in case['rows'])
            return SimpleNamespace(returncode=0,stdout=text,stderr='')
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); exe=root/'mysql.exe'; exe.write_text('fixture'); out=root/'run'
            argv=['mysql_checks','--mysql',str(exe),'--orders-login','orders','--payments-login','payments','--out',str(out),'--confirm-isolated']
            with patch.object(mod.sys,'argv',argv), patch.dict(mod.os.environ,{},clear=True), patch.object(mod,'run',side_effect=backend):
                self.assertEqual(mod.main(),1)
            text=(out/'mysql-direct.json').read_text()
            self.assertNotIn('UNEXPECTED_PRIVATE_OUTPUT',text)
            self.assertFalse(any(q.startswith(('DELETE','DROP')) for q in calls))
            self.assertFalse(json.loads(text)['checks'][-1]['matched_expected'])

    def test_bundle_revision_and_no_overwrite(self):
        mod=load('build_bundle')
        self.assertTrue(hasattr(mod,'build'), 'bundle builder missing')
        import zipfile
        with tempfile.TemporaryDirectory() as tmp:
            out=Path(tmp)/'bundle.zip'
            with self.assertRaises(ValueError): mod.build(out, 'unknown')
            mod.build(out, 'a'*40)
            with zipfile.ZipFile(out) as archive:
                self.assertEqual(archive.read('SOURCE_REVISION.txt').decode().strip(),'a'*40)
                self.assertIn('database-assistant/acceptance/WINDOWS-RUNBOOK.md',archive.namelist())
                self.assertFalse(any('__pycache__' in x or x.endswith('.local.txt') for x in archive.namelist()))
            with self.assertRaises(FileExistsError): mod.build(out, 'a'*40)

if __name__ == '__main__': unittest.main(verbosity=2)
