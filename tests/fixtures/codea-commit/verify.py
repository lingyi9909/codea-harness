#!/usr/bin/env python3
"""Read-only Git outcome oracle. Does not load a Skill or invoke a model."""
import argparse
import base64
import json
import os
import stat
from pathlib import Path
import subprocess

# Inherited GIT_* variables can silently redirect operations to another repository.
def git(repo, *args, check=True):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    env['GIT_TERMINAL_PROMPT'] = '0'
    result = subprocess.run(['git', '-C', str(repo), *args], env=env,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and result.returncode:
        raise RuntimeError(result.stderr.decode('utf-8', 'replace'))
    return result


def encoded(data):
    return base64.b64encode(data).decode('ascii')


def head(repo):
    result = git(repo, 'rev-parse', '--verify', 'HEAD', check=False)
    return result.stdout.decode('ascii').strip() if result.returncode == 0 else None


def branch(repo):
    return git(repo, 'symbolic-ref', '--quiet', '--short', 'HEAD', check=False).stdout.decode('utf-8').strip()


def tree(repo, ref):
    result = {}
    if ref is None:
        return result
    for record in git(repo, 'ls-tree', '-rz', ref).stdout.split(b'\0'):
        if not record:
            continue
        meta, path = record.split(b'\t', 1)
        mode, kind, oid = meta.split()
        # Also preserves gitlink identity without attempting to read a submodule blob.
        result[os.fsdecode(path)] = {'mode': mode.decode(), 'kind': kind.decode(),
                                    'oid': oid.decode()}
    return result


def worktree(repo):
    result = {}
    for parent, dirs, files in os.walk(repo, followlinks=False):
        # Directory symlinks are entries too; record the link, never traverse its target.
        links = [d for d in dirs if d != '.git' and (Path(parent) / d).is_symlink()]
        dirs[:] = [d for d in dirs if d != '.git' and d not in links]
        for name in files + links:
            path = Path(parent) / name
            if path == Path(repo) / '.git':
                continue
            rel = path.relative_to(repo).as_posix()
            is_link = path.is_symlink()
            result[rel] = {'bytes': encoded(os.fsencode(os.readlink(path)) if is_link
                                          else path.read_bytes()), 'symlink': is_link,
                           'executable': bool(path.lstat().st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))
                           if os.name != 'nt' and not is_link else None}
    return result


def operation_state(repo):
    result = {}
    for name in ('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD', 'rebase-merge',
                 'rebase-apply', 'sequencer', 'index.lock'):
        raw = git(repo, 'rev-parse', '--git-path', name).stdout.decode('utf-8').strip()
        path = Path(raw)
        if not path.is_absolute():
            path = Path(repo) / path
        if path.is_file():
            result[name] = encoded(path.read_bytes())
        elif path.is_dir():
            result[name] = {p.relative_to(path).as_posix(): encoded(p.read_bytes())
                            for p in path.rglob('*') if p.is_file()}
    return result


def protection(repo):
    config = git(repo, 'config', '--local', '--null', '--list').stdout
    raw = git(repo, 'config', '--path', '--get', 'core.hooksPath', check=False).stdout
    hook_path = Path(os.fsdecode(raw).strip()) if raw else Path(os.fsdecode(
        git(repo, 'rev-parse', '--git-path', 'hooks').stdout).strip())
    if not hook_path.is_absolute():
        hook_path = Path(repo) / hook_path
    hooks = {p.relative_to(hook_path).as_posix(): encoded(p.read_bytes())
             for p in hook_path.rglob('*') if p.is_file()} if hook_path.is_dir() else {}
    return {'config': encoded(config), 'hooks': hooks}


def snapshot(repo):
    return {'head': head(repo), 'branch': branch(repo),
            'index': encoded(git(repo, 'ls-files', '--stage', '-z').stdout),
            'worktree': worktree(repo), 'operation': operation_state(repo),
            'protection': protection(repo)}


def compare_maps(label, expected, actual):
    errors = []
    for path in sorted(set(expected) | set(actual)):
        if expected.get(path) != actual.get(path):
            errors.append('{}: {} differs (expected {}, actual {})'.format(
                label, path, expected.get(path), actual.get(path)))
    return errors


def verify(repo, manifest):
    data = json.loads(Path(manifest).read_text(encoding='utf-8'))
    if Path(repo).resolve() != Path(data['repo']).resolve():
        return ['repository does not match manifest']
    current = snapshot(repo)
    before = data['before']
    errors = []
    if current['protection'] != before['protection']:
        errors.append('protection: local config or hooks changed')
    if current['branch'] != before['branch']:
        errors.append('branch differs')
    if data['outcome'] == 'stop':
        for key in ('head', 'index', 'operation'):
            if before[key] != current[key]:
                errors.append('{} changed during stop scenario'.format(key))
    else:
        if not current['head'] or current['head'] == before['head']:
            errors.append('HEAD: expected one new commit')
        else:
            parents = git(repo, 'rev-list', '--parents', '-1', 'HEAD').stdout.decode('ascii').split()[1:]
            if parents != ([before['head']] if before['head'] else []):
                errors.append('parents: expected exactly the original HEAD (or root commit)')
            errors += compare_maps('commit tree', data['commit_tree'], tree(repo, 'HEAD'))
        if git(repo, 'ls-files', '--unmerged', '-z').stdout:
            errors.append('index contains conflicts')
        # Compare full index content even if the commit itself was incorrect.
        entries = {}
        for row in git(repo, 'ls-files', '--stage', '-z').stdout.split(b'\0'):
            if row:
                meta, path = row.split(b'\t', 1)
                mode, oid, stage = meta.split()
                entries[os.fsdecode(path)] = {'mode': mode.decode(),
                    'kind': 'commit' if mode == b'160000' else 'blob', 'oid': oid.decode()}
        errors += compare_maps('index', data['commit_tree'], entries)
        if current['operation'] != before['operation']:
            errors.append('operation state differs')
    errors += compare_maps('worktree', data['worktree'], current['worktree'])
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('manifest', type=Path)
    parser.add_argument('repo', type=Path)
    args = parser.parse_args()
    errors = verify(args.repo.resolve(), args.manifest)
    for error in errors:
        print('FAIL:', error)
    print('FAIL' if errors else 'PASS: actual Git boundary matches manifest; message/agent behavior require human review')
    return bool(errors)

if __name__ == '__main__':
    raise SystemExit(main())
