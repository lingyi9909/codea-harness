"""Build an offline source acceptance ZIP from an explicit allowlist, no secrets."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import zipfile

ROOT = Path(__file__).resolve().parents[1]
FILES = '''README.md
agents/database-assistant.md
config/dbhub.example.toml
config/inventory.example.json
scripts/prepare.py
scripts/verify_deployment.py
skills/database-assistant/SKILL.md
skills/database-assistant/references/report-template.md
tests/acceptance.md
tests/deployment-verification.md
tests/test_prepare.py
tests/test_deployment.py
tests/test_acceptance.py
evidence/generated.synthetic.json
evidence/inventory.synthetic.json
evidence/report.md
evidence/automated.txt
evidence/review-regression.txt
evidence/git-scope-regression.txt
acceptance/PLAN.md
acceptance/WINDOWS-RUNBOOK.md
acceptance/stage.py
acceptance/mysql_checks.py
acceptance/observe_timeout.py
acceptance/build_bundle.py
acceptance/Invoke-DBHubCase.ps1
acceptance/cases.json
acceptance/results.template.json
acceptance/sql/01-fixture.sql
acceptance/sql/02-grants.sql'''.splitlines()


def build(out, revision):
    if not re.fullmatch('[0-9a-f]{40}', revision):
        raise ValueError('full source commit required')
    sidecar = out.with_suffix(out.suffix + '.sha256')
    if out.exists() or sidecar.exists(): raise FileExistsError('output already exists')
    entries = {'database-assistant/' + name: (ROOT/name).read_bytes() for name in FILES}
    entries['SOURCE_REVISION.txt'] = (revision + '\n').encode()
    hashes = {name: hashlib.sha256(data).hexdigest() for name, data in sorted(entries.items())}
    entries['SHA256SUMS.json'] = (json.dumps(hashes, indent=2) + '\n').encode()
    with zipfile.ZipFile(out, 'x', compression=zipfile.ZIP_DEFLATED) as archive:
        for name, data in sorted(entries.items()):
            info=zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.compress_type=zipfile.ZIP_DEFLATED
            info.external_attr=0o100644 << 16
            archive.writestr(info, data)
    digest=hashlib.sha256(out.read_bytes()).hexdigest()
    with sidecar.open('x', encoding='utf-8') as stream: stream.write(digest + '  ' + out.name + '\n')
    return digest


if __name__ == '__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--revision',required=True)
    p.add_argument('--out',required=True,type=Path)
    a=p.parse_args()
    print(build(a.out,a.revision))
