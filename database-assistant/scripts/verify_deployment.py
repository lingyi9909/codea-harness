"""Compare independent deployment snapshots; never execute tools or assert T1 PASS.

Observed tools must be normalized from independently captured runtime evidence,
not copied from inventory. This checker cannot authenticate evidence or replace
actual pre-call denial tests. No network or credentials are accepted.
"""
import argparse
from fnmatch import fnmatchcase
import json
from pathlib import Path
import sys
import tomllib
from prepare import prepare, require


def verify(config, inventory, source, candidate, observed, effective):
    expected = prepare(config, inventory, source)
    require(candidate == expected, 'candidate differs from prepared configuration')
    require(set(observed) == {'tools'}, 'invalid observation snapshot')
    registrations = {}
    for row in observed['tools']:
        require(set(row) == {'source', 'kind', 'action', 'evidence_id'}, 'invalid observation fields')
        require(isinstance(row['evidence_id'], str) and row['evidence_id'].strip(), 'missing independent evidence reference')
        action = row['action']
        require(action not in registrations, 'duplicate observed action')
        registrations[action] = (row['source'], row['kind'])
    require(registrations == {t['action']: (t['source'], t['kind']) for t in inventory['tools']},
            'observed registration/source mapping differs from inventory')
    require(isinstance(effective, list), 'expected full native debug agents array')
    agents = {}
    for agent in effective:
        require(agent['id'] not in agents, 'duplicate effective agent')
        agents[agent['id']] = agent
    for name, desired in expected['agents'].items():
        require(name in agents, 'missing effective agent')
        actual = agents[name]
        for field in ('system', 'mode', 'steps'):
            require(actual.get(field) == desired[field], 'effective agent differs from candidate')
        require(not {'prompt', 'permission', 'maxSteps', 'tools'} & actual.keys(), 'legacy agent fields')
        # OpenCode may prepend defaults. The exact trailing deny-all and allowlist
        # must match; any later grant or changed order fails closed.
        rules = actual['permissions']
        require(rules[-len(desired['permissions']):] == desired['permissions'], 'effective permissions mismatch')
    # Conservative static check of other visible agents. Native evaluation and
    # saved/session grants MUST still be tested independently at runtime.
    for name, agent in agents.items():
        if name in expected['agents']:
            continue
        for action in registrations:
            effect = 'allow'  # V2 default is not deny-all.
            for rule in agent['permissions']:
                if fnmatchcase(action, rule['action']) and fnmatchcase('*', rule['resource']):
                    effect = rule['effect']
            require(effect == 'deny', 'another effective agent can access or request DB tools')
    return {'status': 'SNAPSHOT_MATCH_ONLY', 'runtime_acceptance': 'PENDING',
            'required': ['pre-call denial traces', 'all MCP instances and saved grants audit',
                         'MySQL safety and timeout evidence', 'Windows private-model TUI']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('config', 'inventory', 'candidate', 'observed', 'effective'):
        parser.add_argument('--' + name, required=True, type=Path)
    parser.add_argument('--source', required=True)
    args = parser.parse_args()
    try:
        data = {name: json.loads(getattr(args, name).read_text(encoding='utf-8-sig'))
                for name in ('inventory', 'candidate', 'observed', 'effective')}
        result = verify(tomllib.loads(args.config.read_text(encoding='utf-8-sig')),
                        data['inventory'], args.source, data['candidate'], data['observed'], data['effective'])
    except (ValueError, KeyError, TypeError, OSError, AttributeError):
        print('DEPLOYMENT_MISMATCH: incomplete or inconsistent snapshots; do not deploy', file=sys.stderr)
        return 1
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
