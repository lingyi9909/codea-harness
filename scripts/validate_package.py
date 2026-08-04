#!/usr/bin/env python3
from __future__ import annotations
import json
from pathlib import Path
import sys
import yaml
from jsonschema.validators import validator_for

ROOT = Path(__file__).resolve().parents[1]
REQUIRED = [
 'README.md','AGENTS.md','docs/design/codea-harness-v1-design.md','docs/plans/codea-harness-v1-implementation-plan.md',
 'docs/contracts/review-output.schema.json','docs/contracts/test-plan.schema.json','docs/contracts/diagnosis.schema.json','docs/contracts/fix-plan.schema.json',
 'harness/harness.example.yaml','harness/agents/reviewer.md','harness/agents/integration-test-agent.md','harness/agents/runtime-debugger.md','harness/agents/fix-agent.md','harness/tools/README.md',
 *[f'harness/skills/{name}/SKILL.md' for name in ('analyze-change','review-code','design-integration-tests','generate-integration-tests','run-integration-tests','debug-local-service','analyze-failure','fix-bug')]
]

def fail(message: str) -> None:
    raise ValueError(message)

def validate_required() -> None:
    missing=[p for p in REQUIRED if not (ROOT/p).is_file()]
    if missing: fail('missing required files: '+', '.join(missing))

def validate_schemas() -> None:
    for path in sorted((ROOT/'docs/contracts').glob('*.schema.json')):
        schema=json.loads(path.read_text())
        validator_for(schema).check_schema(schema)

def validate_yaml() -> None:
    data=yaml.safe_load((ROOT/'harness/harness.example.yaml').read_text())
    if data.get('version') != 1: fail('version must be 1')
    if data.get('project',{}).get('type') != 'maven': fail('project.type must be maven')
    for section in ('integrationTest','service','scope','write','runs'):
        if section not in data: fail(f'missing YAML section: {section}')
    if not isinstance(data['integrationTest'].get('args'), list): fail('integrationTest.args must be a list')
    if not isinstance(data['service'].get('args'), list): fail('service.args must be a list')
    denied=set(data['write'].get('deniedPaths',[]))
    if '.git/**' not in denied or '.github/**' not in denied: fail('deniedPaths must protect .git and .github')
    if data['service'].get('readiness',{}).get('type') not in {'log','http'}: fail('readiness.type must be log or http')

def main() -> int:
    try:
        validate_required(); validate_schemas(); validate_yaml()
    except Exception as exc:
        print(f'INVALID: {exc}', file=sys.stderr); return 1
    print(f'VALID: {len(REQUIRED)} required files, 4 schemas, safe example configuration')
    return 0
if __name__ == '__main__': raise SystemExit(main())
