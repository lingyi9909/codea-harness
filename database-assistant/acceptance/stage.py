"""Stage a NEW isolated OpenCode project; never edit global/user configuration."""
import argparse
import copy
import json
from pathlib import Path
import re
import shutil
import sys

ROOT = Path(__file__).resolve().parents[1]
DENY = {'action': '*', 'resource': '*', 'effect': 'deny'}


def stage(target, base, candidate, official):
    if base.get('permissions') != [DENY] or base.get('agents'):
        raise ValueError('isolated base must deny all and contain no agents')
    if set(candidate) != {'agents'} or set(candidate['agents']) != {'database-assistant', 'database-entry'}:
        raise ValueError('expected prepared agent pair')
    for name, agent in candidate['agents'].items():
        if set(agent) != {'description', 'mode', 'system', 'steps', 'permissions'}:
            raise ValueError('invalid V2 candidate fields')
        if not agent['system'] or agent['steps'] != 8 or agent['permissions'][0] != DENY:
            raise ValueError('invalid V2 candidate policy')
        if agent['mode'] != ('subagent' if name == 'database-assistant' else 'primary'):
            raise ValueError('invalid agent mode')
    if not re.search(r'^name: dbhub\s*$', (official / 'SKILL.md').read_text(encoding='utf-8'), re.M):
        raise ValueError('official dbhub Skill required')
    for folder in (official, ROOT / 'skills/database-assistant'):
        if folder.is_symlink() or any(p.is_symlink() for p in folder.rglob('*')):
            raise ValueError('Skill symlinks are not supported')
    # All validation precedes exclusive creation. Interrupted copies are NOT installed.
    target.mkdir(parents=False, exist_ok=False)
    config = copy.deepcopy(base)
    config['agents'] = candidate['agents']
    (target / 'opencode.json').write_text(json.dumps(config, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    skills = target / '.opencode/skills'
    skills.mkdir(parents=True)
    shutil.copytree(ROOT / 'skills/database-assistant', skills / 'database-assistant')
    shutil.copytree(official, skills / 'dbhub')
    (target / 'STAGING_COMPLETE.txt').write_text('STAGED ONLY; verify actual effective config and runtime permissions before tests.\n', encoding='utf-8')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ('target', 'base', 'candidate', 'official-skill'):
        p.add_argument('--' + name, required=True, type=Path)
    a = p.parse_args()
    try:
        stage(a.target, json.loads(a.base.read_text(encoding='utf-8-sig')),
              json.loads(a.candidate.read_text(encoding='utf-8-sig')), a.official_skill)
    except (ValueError, KeyError, TypeError, OSError, IndexError):
        print('STAGING_FAILED: check inputs or use a new empty target; no existing configuration overwritten', file=sys.stderr)
        return 1
    print('STAGED_ONLY: not an installation or safety PASS; run in target and verify effective settings')
    return 0


if __name__ == '__main__': raise SystemExit(main())
