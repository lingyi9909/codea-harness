"""TEST ONLY: fixed synthetic MySQL acceptance cases, no Agent integration.

Credentials stay in mysql_config_editor login paths. Every run requires explicit
isolated-server acknowledgement and both canary views before negative tests.
"""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parent
PERMISSION_CODES = {1044, 1142, 1143}


def command(mysql, login, sql):
    if not re.fullmatch(r'[A-Za-z][A-Za-z0-9_-]{0,63}', login):
        raise ValueError('invalid login path')
    return [str(mysql), '--login-path=' + login, '--connect-timeout=5',
            '--batch', '--raw', '--skip-column-names', '--default-character-set=utf8mb4', '--execute', sql]


def error_code(stderr):
    found = re.search(r'ERROR\s+(\d+)', stderr)
    return int(found.group(1)) if found else None


def assess(case, code, stdout, stderr):
    if case['expect'] == 'denied':
        return code != 0 and error_code(stderr) in PERMISSION_CODES
    if case['expect'] == 'error':
        return code != 0 and error_code(stderr) in case['codes']
    rows = [line.split('\t') for line in stdout.splitlines()]
    return code == 0 and rows == case['rows']


def run(mysql, login, sql):
    # Never shell=True; SQL and login path are separate argv values.
    return subprocess.run(command(mysql, login, sql), capture_output=True,
                          text=True, encoding='utf-8', errors='replace', timeout=20)


def save(path, value):
    with path.open('x', encoding='utf-8') as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.write('\n')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--mysql', required=True, type=Path)
    p.add_argument('--orders-login', required=True)
    p.add_argument('--payments-login', required=True)
    p.add_argument('--out', required=True, type=Path)
    p.add_argument('--confirm-isolated', action='store_true')
    a = p.parse_args()
    if not a.confirm_isolated or os.environ.get('MYSQL_PWD') or not a.mysql.is_file():
        p.error('isolated acknowledgement, mysql executable and login-path credentials required; do not set MYSQL_PWD')
    # Refuse reuse: preserve original run evidence.
    a.out.mkdir(parents=False, exist_ok=False)
    report = {'environment': 'C-candidate', 'layer': 'MySQL direct (not DBHub/TUI)',
              'started_utc': datetime.now(timezone.utc).isoformat(), 'stage_acceptance': 'PENDING', 'checks': []}
    try:
        for login, db, marker in [(a.orders_login, 'orders', 'ORDERS'), (a.payments_login, 'payments', 'PAYMENTS')]:
            sql = f'SELECT marker FROM dbassist_t1_{db}.v_canary'
            result = run(a.mysql, login, sql)
            if result.returncode != 0 or result.stdout.strip() != f'DBASSIST_T1_SYNTHETIC_{marker}_V1':
                raise ValueError('canary mismatch; no attack queries executed')
        version = run(a.mysql, a.orders_login, 'SELECT VERSION()')
        if version.returncode != 0: raise ValueError('version capture failed')
        report['mysql_version'] = version.stdout.strip()
        for case in json.loads((ROOT / 'cases.json').read_text(encoding='utf-8')):
            if case['expect'] == 'denied':
                probe = run(a.mysql, a.orders_login, 'SELECT id,value FROM dbassist_t1_orders.write_probe ORDER BY id')
                if probe.returncode != 0 or probe.stdout.strip() != '1\t123':
                    raise ValueError('probe changed or unreadable; stop immediately')
            started = time.monotonic()
            result = run(a.mysql, a.orders_login, case['sql'])
            passed = assess(case, result.returncode, result.stdout, result.stderr)
            row = {'id': case['id'], 'sql': case['sql'], 'matched_expected': passed,
                   'exit_code': result.returncode, 'mysql_error_code': error_code(result.stderr),
                   'elapsed_seconds': round(time.monotonic()-started, 3)}
            # Only persist expected synthetic public values, never unexpected/sensitive output.
            if passed and case['expect'] == 'rows': row['rows'] = case['rows']
            report['checks'].append(row)
            if not passed: raise ValueError('unexpected result; stop immediately, DBA inspect fixture')
            if case['expect'] == 'denied':
                probe = run(a.mysql, a.orders_login, 'SELECT id,value FROM dbassist_t1_orders.write_probe ORDER BY id')
                unchanged = probe.returncode == 0 and probe.stdout.strip() == '1\t123'
                row['probe_unchanged'] = unchanged
                if not unchanged: raise ValueError('probe changed after denied query')
        # Reverse-direction isolation is independent of orders -> payments denial.
        case = {'expect': 'denied'}
        sql = 'SELECT id FROM dbassist_t1_orders.v_orders'
        result = run(a.mysql, a.payments_login, sql)
        passed = assess(case, result.returncode, result.stdout, result.stderr)
        report['checks'].append({'id': 'T12-reverse', 'sql': sql, 'matched_expected': passed,
                                 'mysql_error_code': error_code(result.stderr)})
        if not passed: raise ValueError('reverse isolation failed')
        report['direct_checks'] = 'MATCHED_EXPECTATIONS_ONLY'
    except subprocess.TimeoutExpired:
        report['failure'] = 'CLIENT_WAIT_EXPIRED_SERVER_STATE_UNKNOWN; DBA must inspect server'
    except (ValueError, OSError):
        report['failure'] = 'CHECK_FAILED; stop, inspect local client/server, do not reuse fixture'
    finally:
        save(a.out / 'mysql-direct.json', report)
    print('DIRECT_CHECKS_ONLY; T1 remains PENDING; inspect mysql-direct.json')
    return 1 if 'failure' in report else 0


if __name__ == '__main__': raise SystemExit(main())
