"""Offline candidate config generator. Never connects to DBHub or executes SQL.

Input inventory is an administrator-reviewed export of actual OpenCode permission
ACTION names, NOT an LLM response or a guessed MCP naming convention.
"""
import argparse
import json
from pathlib import Path
import re
import sys
import tomllib

IDENTIFIER = re.compile(r'[A-Za-z][A-Za-z0-9_-]{0,127}\Z')
ENV = re.compile(r'\$\{[A-Z][A-Z0-9_]*\}\Z')
RESERVED = {'shell', 'read', 'edit', 'write', 'patch', 'glob', 'grep', 'skill',
            'subagent', 'question', 'webfetch', 'websearch', 'external_directory',
            'bash', 'task', 'permission', 'mcp', 'todoread', 'todowrite', 'codesearch'}
KINDS = {'search_objects', 'execute_sql'}
ROOT = Path(__file__).resolve().parents[1]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def bounded(value, maximum):
    return type(value) is int and 1 <= value <= maximum


def prepare(config, inventory, source):
    require(set(config) == {'sources', 'tools'}, 'unsupported TOML sections')
    require(isinstance(config['sources'], list) and config['sources'], 'sources required')
    sources = {}
    for item in config['sources']:
        require(set(item) == {'id', 'dsn', 'query_timeout', 'connection_timeout'}, 'unsupported source fields')
        name = item['id']
        require(isinstance(name, str) and IDENTIFIER.fullmatch(name) and name not in sources, 'invalid or duplicate source')
        require(isinstance(item['dsn'], str) and ENV.fullmatch(item['dsn']), 'DSN must be a whole environment reference')
        require(bounded(item['query_timeout'], 15), 'query timeout must be 1..15 seconds')
        require(bounded(item['connection_timeout'], 60), 'connection timeout must be 1..60 seconds')
        sources[name] = item
    require(source in sources, 'source is not configured')
    configured = set()
    for item in config['tools']:
        require(set(item) == {'name', 'source', 'readonly', 'max_rows'}, 'unsupported tool fields')
        require(item['name'] == 'execute_sql', 'only execute_sql overrides supported in T1')
        require(item['source'] in sources and item['source'] not in configured, 'unknown or duplicate tool source')
        require(item['readonly'] is True, 'readonly must be true')
        require(bounded(item['max_rows'], 100), 'max_rows must be 1..100')
        configured.add(item['source'])
    require(configured == set(sources), 'each source requires readonly execute_sql configuration')
    require(set(inventory) == {'opencode_version', 'dbhub_version', 'tools'}, 'invalid inventory fields')
    require(isinstance(inventory['opencode_version'], str) and re.fullmatch(r'2\.\d+\.\d+', inventory['opencode_version']), 'OpenCode V2 version required')
    require(isinstance(inventory['dbhub_version'], str) and inventory['dbhub_version'].strip(), 'DBHub version required')
    actions, pairs, selected = set(), set(), []
    for item in inventory['tools']:
        require(set(item) == {'source', 'kind', 'action'}, 'invalid inventory tool fields')
        action = item['action']
        require(isinstance(action, str) and IDENTIFIER.fullmatch(action) and '_' in action and action not in RESERVED,
                'invalid MCP permission action')
        require(action not in actions, 'duplicate permission action')
        pair = (item['source'], item['kind'])
        require(item['source'] in sources and item['kind'] in KINDS and pair not in pairs, 'invalid source/kind mapping')
        actions.add(action); pairs.add(pair)
        if item['source'] == source:
            selected.append(action)
    require(pairs == {(s, k) for s in sources for k in KINDS}, 'inventory must include both tools for every configured source')
    deny = {'action': '*', 'resource': '*', 'effect': 'deny'}
    permissions = [deny.copy(),
                   {'action': 'skill', 'resource': 'database-assistant', 'effect': 'allow'},
                   {'action': 'skill', 'resource': 'dbhub', 'effect': 'allow'}]
    permissions.extend({'action': a, 'resource': '*', 'effect': 'allow'} for a in sorted(selected))
    prompt = (ROOT / 'agents' / 'database-assistant.md').read_text(encoding='utf-8').split('---', 2)[2].strip()
    return {'agents': {
        'database-assistant': {'description': '受限只读数据库查询助手', 'mode': 'subagent',
                               'permissions': permissions,
                               'prompt': prompt + '\n本部署唯一授权数据源 ID：' + source},
        'database-entry': {'description': '数据库助手受限委派入口', 'mode': 'primary',
                           'permissions': [deny.copy(), {'action': 'subagent', 'resource': 'database-assistant', 'effect': 'allow'}],
                           'prompt': '仅委派 database-assistant。传递用户问题、明确数据源、条件、时间范围及证据 ID；缺失时先在对话中澄清。不得直接查询数据库。每次重新委派重新验证数据源范围。'}
    }}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', required=True, type=Path)
    parser.add_argument('--inventory', required=True, type=Path)
    parser.add_argument('--source', required=True)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    try:
        result = prepare(tomllib.loads(args.config.read_text(encoding='utf-8')),
                         json.loads(args.inventory.read_text(encoding='utf-8')), args.source)
        # Exclusive creation: never overwrite active settings or another candidate.
        with args.out.open('x', encoding='utf-8') as stream:
            json.dump(result, stream, ensure_ascii=False, indent=2)
            stream.write('\n')
    except (ValueError, KeyError, TypeError, OSError):
        print('PREPARE_FAILED: invalid input or output unavailable; no deployment performed', file=sys.stderr)
        return 1
    print('CANDIDATE_ONLY: verify effective permissions in isolated environment before deployment')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
