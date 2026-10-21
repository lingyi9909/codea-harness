#!/usr/bin/env python3
"""Create synthetic acceptance data in a new temporary Git repository only."""
import argparse
import json
import os
from pathlib import Path
import shutil
import tempfile

from verify import git, snapshot, tree, worktree, encoded

CASES = {case: 'commit' for case in ('C01', 'C02', 'C03', 'C07', 'C08',
    'C10-message', 'C10-index', 'C10-post', 'C14', 'C15-binary', 'C15-worktree', 'C15-large')}
CASES.update({case: 'stop' for case in ('C04', 'C05', 'C06', 'C09-hook', 'C09-identity',
    'C09-sign', 'C09-lock', 'C11-detached', 'C11-merge', 'C11-rebase',
    'C11-cherry-pick', 'C11-revert', 'C13-args', 'C14-staged-report')})


def write(repo, path, data):
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data if isinstance(data, bytes) else data.encode('utf-8'))


def add(repo, *paths):
    git(repo, '--literal-pathspecs', 'add', '--', *paths)


def commit(repo, message):
    # Fixture initialization only. Product commit execution is never done by this file.
    git(repo, 'commit', '-m', message)


def hook(repo, name, content):
    path = repo / '.git' / 'hooks' / name
    path.write_bytes(('#!/bin/sh\n' + content + '\n').encode('utf-8'))
    path.chmod(0o755)


def create(case, destination):
    if case not in CASES:
        raise ValueError('unknown scenario: ' + case)
    destination = Path(destination).resolve()
    if destination.exists() and (not destination.is_dir() or any(destination.iterdir())):
        raise ValueError('destination must be a new or empty directory')
    manifest = destination.parent / (destination.name + '.manifest.json')
    if manifest.exists():
        raise ValueError('manifest already exists; use a fresh destination')
    if case == 'C15-worktree' and (destination.parent / (destination.name + '-linked')).exists():
        raise ValueError('linked worktree destination already exists')
    destination.mkdir(parents=True, exist_ok=True)
    git(destination, 'init', '-b', 'fixture')
    # Repository-local settings isolate tests from signing/hooks/filters in user config.
    for key, value in {'user.name': 'Codea Fixture', 'user.email': 'fixture@example.invalid',
                       'commit.gpgSign': 'false', 'core.autocrlf': 'false',
                       'core.hooksPath': str(destination / '.git' / 'hooks'),
                       'core.fsmonitor': 'false'}.items():
        git(destination, 'config', '--local', key, value)
    repo = destination
    write(repo, 'task.txt', 'before\n')
    write(repo, 'delete.txt', 'delete me\n')
    write(repo, 'rename-old.txt', 'rename me\n')
    write(repo, 'shared.txt', b'A=0\n' + b'padding\n' * 12 + b'B=0\n')
    if case != 'C07':
        add(repo, 'task.txt', 'delete.txt', 'rename-old.txt', 'shared.txt')
        commit(repo, 'chore(fixture): baseline')
    if case in ('C11-merge', 'C11-rebase', 'C11-cherry-pick', 'C11-revert'):
        git(repo, 'switch', '-c', 'side')
        write(repo, 'task.txt', 'side\n')
        add(repo, 'task.txt')
        commit(repo, 'feat(fixture): side')
        side = git(repo, 'rev-parse', 'HEAD').stdout.decode().strip()
        git(repo, 'switch', 'fixture')
        write(repo, 'task.txt', 'main\n')
        add(repo, 'task.txt')
        commit(repo, 'feat(fixture): main')
        if case == 'C11-revert':
            write(repo, 'task.txt', 'later\n')
            add(repo, 'task.txt')
            commit(repo, 'feat(fixture): later')
            git(repo, 'revert', '--no-edit', 'HEAD~1', check=False)
        else:
            command = {'C11-merge': ('merge', 'side'), 'C11-rebase': ('rebase', 'side'),
                       'C11-cherry-pick': ('cherry-pick', side)}[case]
            git(repo, *command, check=False)
    elif case == 'C02':
        write(repo, 'shared.txt', b'A=1\n' + b'padding\n' * 12 + b'B=0\n')
        add(repo, 'shared.txt')
        write(repo, 'shared.txt', b'A=1\n' + b'padding\n' * 12 + b'B=1\n')
        write(repo, 'other.txt', 'unrelated\n')
    elif case == 'C03':
        write(repo, 'task.txt', 'after\n')
        write(repo, 'new.txt', 'new\n')
        (repo / 'delete.txt').unlink()
        write(repo, 'unrelated.txt', 'leave me\n')
    elif case in ('C04', 'C05'):
        write(repo, 'task.txt', 'task one\n')
        write(repo, 'unrelated.txt', 'another task\n')
        if case == 'C05':
            add(repo, 'task.txt', 'unrelated.txt')
    elif case == 'C06':
        pass
    elif case == 'C14':
        write(repo, '.gitignore', '*.tmp\n')
        write(repo, '.github/config.yml', 'enabled: true\n')
        write(repo, 'temp/codea-review-report.md', 'temporary review\n')
    elif case == 'C08':
        # All are legal on Windows. Quotes/backticks are legal only on POSIX.
        names = ['中文 空格.txt', '$dollar.txt', '-leading.txt', '[literal]*.txt'] if os.name != 'nt' else ['中文 空格.txt', '$dollar.txt', '-leading.txt', '[literal].txt']
        if os.name != 'nt':
            names += ['quote\'".txt', '`touch SENTINEL`.txt']
        for name in names:
            write(repo, name, '路径保真\n')
        add(repo, *names)
    elif case == 'C15-binary':
        write(repo, 'asset.bin', b'\0\xff\x01\x00new binary\n')
        add(repo, 'asset.bin')
    elif case == 'C15-large':
        write(repo, 'large.txt', ''.join('line {}\n'.format(i) for i in range(20000)))
        add(repo, 'large.txt')
    else:
        write(repo, 'task.txt', 'after\n')
        add(repo, 'task.txt')
        if case == 'C01':
            git(repo, 'mv', '--', 'rename-old.txt', 'rename-new.txt')
            git(repo, 'rm', '--', 'delete.txt')
    if case == 'C07':
        add(repo, 'task.txt')
    if case == 'C09-hook':
        hook(repo, 'pre-commit', 'echo "fixture rejects commit" >&2\nexit 1')
    elif case == 'C09-identity':
        git(repo, 'config', '--local', 'user.name', '')
        git(repo, 'config', '--local', 'user.email', '')
        git(repo, 'config', '--local', 'user.useConfigOnly', 'true')
    elif case == 'C09-sign':
        git(repo, 'config', '--local', 'commit.gpgSign', 'true')
        git(repo, 'config', '--local', 'gpg.program', str(repo / '.git' / 'nonexistent-signing-program'))
    elif case == 'C09-lock':
        (repo / '.git' / 'index.lock').write_bytes(b'fixture lock\n')
    elif case == 'C10-message':
        hook(repo, 'commit-msg', 'printf "fix(fixture): hook 改写消息\\n" > "$1"')
    elif case == 'C10-index':
        hook(repo, 'pre-commit', 'printf "hook changed content\\n" > task.txt\ngit --literal-pathspecs add -- task.txt')
    elif case == 'C10-post':
        hook(repo, 'post-commit', 'printf "post-hook\\n" > post.txt\nexit 1')
    elif case == 'C11-detached':
        git(repo, 'checkout', '--detach')
    elif case == 'C14-staged-report':
        write(repo, 'temp/codea-review-report.md', 'temporary review\n')
        add(repo, 'temp/codea-review-report.md')
    if case == 'C15-worktree':
        linked = destination.parent / (destination.name + '-linked')
        if linked.exists():
            raise ValueError('linked worktree destination already exists')
        # The main fixture remains dirty; the linked worktree has its own index.
        git(repo, 'worktree', 'add', '-b', 'linked', str(linked), 'HEAD')
        repo = linked
        write(repo, 'task.txt', 'after\n')
        add(repo, 'task.txt')
    before = snapshot(repo)
    expected = tree(repo, git(repo, 'write-tree').stdout.decode().strip()) if not (
        git(repo, 'ls-files', '--unmerged', '-z').stdout or case == 'C09-lock') else {}
    if case in ('C03', 'C14'):
        # Build the oracle in a separate temporary index; never pre-stage the task for the model.
        with tempfile.TemporaryDirectory(prefix='codea-oracle-') as tmp:
            alt = Path(tmp) / 'index'
            real = git(repo, 'rev-parse', '--git-path', 'index').stdout.decode().strip()
            real = Path(real) if Path(real).is_absolute() else repo / real
            shutil.copyfile(real, alt)
            import subprocess
            env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
            env['GIT_INDEX_FILE'] = str(alt)
            paths = ['task.txt', 'new.txt', 'delete.txt'] if case == 'C03' else ['.gitignore', '.github/config.yml']
            subprocess.run(['git', '-C', str(repo), '--literal-pathspecs', 'add', '--', *paths], env=env, check=True)
            oid = subprocess.check_output(['git', '-C', str(repo), 'write-tree'], env=env).decode().strip()
            expected = tree(repo, oid)
    expected_worktree = worktree(repo)
    if case == 'C10-post':
        expected_worktree['post.txt'] = {'bytes': encoded(b'post-hook\n'), 'symlink': False,
                                          'executable': False if os.name != 'nt' else None}
    data = {'schema': 1, 'case': case, 'repo': str(repo), 'outcome': CASES[case],
            'before': before, 'commit_tree': expected, 'worktree': expected_worktree}
    manifest = destination.parent / (destination.name + '.manifest.json')
    # Exclusive creation prevents silently replacing evidence from an earlier run.
    with manifest.open('x', encoding='utf-8') as stream:
        json.dump(data, stream, ensure_ascii=True, indent=2)
    return repo, manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('case', choices=sorted(CASES))
    parser.add_argument('destination', type=Path)
    args = parser.parse_args()
    repo, manifest = create(args.case, args.destination)
    print(json.dumps({'repo': str(repo), 'manifest': str(manifest)}, ensure_ascii=True))

if __name__ == '__main__':
    main()
