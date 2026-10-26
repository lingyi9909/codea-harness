#!/usr/bin/env python3
"""Record an external acceptance checkpoint; never executes or authorizes a commit."""
import argparse
import json
from pathlib import Path

from verify import git, snapshot, index_tree


def record(repo, manifest, expectation, message_file=None):
    repo = Path(git(Path(repo).resolve(), 'rev-parse', '--show-toplevel').stdout.decode('utf-8').rstrip('\r\n')).resolve()
    manifest = Path(manifest).resolve()
    if repo == manifest or repo in manifest.parents:
        raise ValueError('checkpoint must be outside the test repository')
    if expectation == 'commit' and message_file is None:
        raise ValueError('commit checkpoint requires the full confirmed message file')
    before = snapshot(repo)
    data = {'schema': 1, 'case': 'preview-checkpoint', 'repo': str(repo),
            'outcome': 'stop' if expectation == 'unchanged' else 'commit',
            'before': before, 'commit_tree': index_tree(repo), 'worktree': before['worktree']}
    if expectation == 'commit':
        data['confirmed_message'] = Path(message_file).read_bytes().decode('utf-8', errors='strict')
    # Never overwrite prior evidence, and never write into the repository.
    with manifest.open('x', encoding='utf-8') as stream:
        json.dump(data, stream, ensure_ascii=True, indent=2)
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('repo', type=Path)
    parser.add_argument('manifest', type=Path)
    parser.add_argument('--expect', choices=['unchanged', 'commit'], required=True)
    parser.add_argument('--message-file', type=Path)
    args = parser.parse_args()
    print(record(args.repo, args.manifest, args.expect, args.message_file))


if __name__ == '__main__':
    main()
