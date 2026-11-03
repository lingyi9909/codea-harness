import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, realpathSync } from 'node:fs';
import { isAbsolute, relative, resolve } from 'node:path';

const OID = /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;

function git(directory: string, args: string[]): string {
  return execFileSync('git', ['-C', directory, ...args], {
    encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'],
    timeout: 15000, maxBuffer: 16 * 1024 * 1024,
    env: { ...process.env, GIT_TERMINAL_PROMPT: '0', GIT_OPTIONAL_LOCKS: '0' },
  }).trim();
}
function rawGit(directory: string, args: string[]): Buffer {
  return execFileSync('git', ['-C', directory, ...args], {
    stdio: ['ignore', 'pipe', 'ignore'], timeout: 15000,
    maxBuffer: 16 * 1024 * 1024,
    env: { ...process.env, GIT_TERMINAL_PROMPT: '0', GIT_OPTIONAL_LOCKS: '0' },
  });
}
function isWithin(parent: string, child: string): boolean {
  const rel = relative(parent, child);
  return rel === '' || (!rel.startsWith('..') && !isAbsolute(rel));
}
function mustOID(label: string, value: string): void {
  if (!OID.test(value)) throw new Error('INVALID_' + label + '_OID');
}
function refusePartialGit(root: string): void {
  for (const args of [
    ['config', '--get', 'extensions.partialclone'],
    ['config', '--name-only', '--get-regexp', 'promisor'],
  ]) {
    try { git(root, args); throw new Error('PARTIAL_OR_PROMISOR_REPOSITORY_FORBIDDEN'); }
    catch (error) {
      if (error instanceof Error && error.message === 'PARTIAL_OR_PROMISOR_REPOSITORY_FORBIDDEN')
        throw error;
      const status = (error as { status?: number }).status;
      if (status !== 1) throw new Error('GIT_CONFIG_PROBE_FAILED');
    }
  }
}

export type FrozenSource = { headOID: string; baseOID: string; snapshotRoot: string; originalRoot: string };

export function inspectFrozenSnapshot(input: {
  sourceRoot: string; snapshotRoot: string; expectedHeadOID: string; expectedMergeBaseOID: string;
}): FrozenSource {
  mustOID('HEAD', input.expectedHeadOID);
  mustOID('MERGE_BASE', input.expectedMergeBaseOID);
  const original = realpathSync(resolve(input.sourceRoot));
  const snapshot = realpathSync(resolve(input.snapshotRoot));
  if (isWithin(original, snapshot) || isWithin(snapshot, original))
    throw new Error('SNAPSHOT_NOT_INDEPENDENT');
  if (git(snapshot, ['rev-parse', '--show-prefix']) !== '')
    throw new Error('SNAPSHOT_NOT_GIT_ROOT');
  if (git(original, ['rev-parse', '--show-prefix']) !== '')
    throw new Error('SOURCE_NOT_GIT_ROOT');
  // Worktrees sharing source's .git common storage do not qualify as independent copies.
  const sharedGit = realpathSync(git(snapshot, ['rev-parse', '--path-format=absolute', '--git-common-dir']));
  if (!isWithin(snapshot, sharedGit)) throw new Error('SNAPSHOT_USES_EXTERNAL_GIT_COMMON_DIR');
  refusePartialGit(snapshot);
  const head = git(snapshot, ['rev-parse', '--verify', 'HEAD']);
  mustOID('ACTUAL_HEAD', head);
  const base = git(snapshot, ['show-ref', '--verify', '--hash', 'refs/heads/harness-review-base']);
  mustOID('ACTUAL_BASE', base);
  const mergeBase = git(snapshot, ['merge-base', base, 'HEAD']);
  mustOID('ACTUAL_MERGE_BASE', mergeBase);
  if (head !== input.expectedHeadOID || base !== input.expectedMergeBaseOID ||
      mergeBase !== input.expectedMergeBaseOID)
    throw new Error('FROZEN_GIT_SCOPE_MISMATCH');
  if (rawGit(snapshot, ['status', '--porcelain=v1', '-z', '--untracked-files=all']).length)
    throw new Error('SNAPSHOT_NOT_CLEAN');
  return {
    headOID: head, baseOID: base, snapshotRoot: snapshot, originalRoot: original,
  };
}

/** Read-only original fingerprint; must be re-observed after the model has terminated. */
export function originalRepositoryFingerprint(sourceRoot: string): string {
  const root = realpathSync(resolve(sourceRoot));
  const head = git(root, ['rev-parse', '--verify', 'HEAD']);
  const index = git(root, ['rev-parse', '--path-format=absolute', '--git-path', 'index']);
  const hash = createHash('sha256').update(head);
  if (existsSync(index)) hash.update(readFileSync(index));
  hash.update(rawGit(root, ['status', '--porcelain=v1', '-z', '--untracked-files=all']));
  hash.update(rawGit(root, ['diff', '--no-ext-diff', '--no-textconv', '--binary', 'HEAD', '--']));
  return hash.digest('hex');
}
