"""TEST ONLY. Observe a marked query from a separate DBA login; never kill SQL.

No automatic T13 verdict: client error, early server termination, pool state and
cancel-failure evidence must be correlated by the tester and independent reviewer.
"""
import argparse
from datetime import datetime, timezone
import json
import os
import re
import time
from mysql_checks import run


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--mysql', required=True)
    p.add_argument('--observer-login', required=True)
    p.add_argument('--run-id', required=True)
    p.add_argument('--seconds', type=int, default=30)
    p.add_argument('--out', required=True)
    p.add_argument('--confirm-isolated', action='store_true')
    a = p.parse_args()
    if not a.confirm_isolated or not re.fullmatch('[A-Za-z0-9_]{1,24}', a.run_id) or not 10 <= a.seconds <= 120 or os.environ.get('MYSQL_PWD'):
        p.error('isolated acknowledgement, safe run ID, 10..120 seconds and login-path credentials required')
    marker = 'DBASSIST_T1_TIMEOUT_' + a.run_id
    sql = ("SELECT ID,TIME,COALESCE(STATE,'') FROM information_schema.PROCESSLIST "
           "WHERE ID<>CONNECTION_ID() AND LOCATE('" + marker + "',INFO)>0 ORDER BY ID")
    deadline = time.monotonic() + a.seconds
    with open(a.out, 'x', encoding='utf-8') as stream:
        while time.monotonic() < deadline:
            try:
                r = run(a.mysql, a.observer_login, sql)
                if r.returncode: raise ValueError('observer failed')
                record = {'utc': datetime.now(timezone.utc).isoformat(), 'marker': marker,
                          'threads': [line.split('\t') for line in r.stdout.splitlines()], 'verdict': 'OBSERVATION_ONLY'}
            except Exception:
                record = {'utc': datetime.now(timezone.utc).isoformat(), 'verdict': 'OBSERVER_FAILED_SERVER_STATE_UNKNOWN'}
                stream.write(json.dumps(record) + '\n'); stream.flush()
                return 1
            stream.write(json.dumps(record) + '\n'); stream.flush()
            time.sleep(0.5)
    print('OBSERVATION_ONLY: absent rows without a prior observed running thread prove nothing')
    return 0


if __name__ == '__main__': raise SystemExit(main())
