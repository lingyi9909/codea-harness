import { readFile, writeFile, mkdir, realpath } from 'node:fs/promises';
import { dirname, isAbsolute, relative, resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { evaluateCapabilities, type Observation } from '../packages/opencode-adapter/src/probe.ts';
import { LocalNativeTransport, verifyWrittenReport, type NativeWireManifest } from '../packages/opencode-adapter/src/native.ts';
import { assertNativeId, reviewArguments, REVIEW_COMMAND } from '../packages/opencode-adapter/src/mapping.ts';
import { inspectFrozenSnapshot, originalRepositoryFingerprint } from '../packages/opencode-adapter/src/git-guard.ts';

type CompanyManifest = NativeWireManifest & {
  responseFields: { build: string; identity: string; sessionId: string; sessionLocation: string };
  requestFields: { sessionLocation: string; commandName: string; commandText: string };
};

function argumentsMap(argv: string[]): Map<string, string | true> {
  const args = new Map<string, string | true>();
  const flags = new Set(['--windows-smoke', '--run-review', '--verify-report']);
  const named = new Set(['--manifest', '--snapshot', '--source', '--out', '--controller', '--run-started-at', '--expected-head', '--expected-merge-base', '--before-fingerprint']);
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (args.has(arg)) throw new Error('DUPLICATE_ARGUMENT: ' + arg);
    if (flags.has(arg)) args.set(arg, true);
    else if (named.has(arg) && argv[i + 1] && !argv[i + 1].startsWith('--')) args.set(arg, argv[++i]);
    else throw new Error('INVALID_ARGUMENT: ' + arg);
  }
  return args;
}
function getValue(args: Map<string, string | true>, name: string): string | undefined {
  const value = args.get(name);
  return typeof value === 'string' ? value : undefined;
}
function field(obj: unknown, key: string): unknown {
  if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(key) || !obj || typeof obj !== 'object') return undefined;
  return (obj as Record<string, unknown>)[key];
}
function matchesPath(source: string, path: string): boolean {
  const rel = relative(source, path);
  return rel === '' || (!rel.startsWith('..') && !isAbsolute(rel));
}

async function main() {
  const args = argumentsMap(process.argv.slice(2));
  const runId = randomUUID();
  if (args.has('--run-review') && process.platform !== 'win32')
    throw new Error('REAL_REVIEW_MUST_RUN_ON_COMPANY_WINDOWS');
  const outPath = resolve(getValue(args, '--out') ?? 'probe-output/t01-result.json');
  if (args.has('--windows-smoke') && process.platform !== 'win32')
    throw new Error('WINDOWS_NATIVE_TEST_REQUIRED');

  const observations: Record<string, Observation> = {};
  const record = (name: string, status: Observation['status'], evidence: string) => {
    observations[name] = { status, evidence, windowsRunId: process.platform === 'win32' ? runId : undefined };
  };
  const manifestPath = getValue(args, '--manifest');
  if (!manifestPath) throw new Error('COMPANY_MANIFEST_REQUIRED: pass --manifest <local-json>');
  const manifest = JSON.parse(await readFile(resolve(manifestPath), 'utf8')) as CompanyManifest;
  const client = new LocalNativeTransport(manifest, process.env.CODE_HARNESS_NATIVE_TOKEN);

  const response = await client.call('identity', 'GET');
  if (response.status !== 200 || field(response.body, manifest.responseFields.build) !== manifest.build ||
    field(response.body, manifest.responseFields.identity) !== manifest.serviceIdentity)
    record('service.identity', 'UNSUPPORTED', 'Real identity/build mismatch, HTTP ' + response.status);
  else record('service.identity', 'SUPPORTED', 'Live loopback OpenCode identity/build matched pinned manifest');
  // An HTTP identity response does not prove that authentication is required.
  if (args.has('--run-review') && observations['service.identity'].status !== 'SUPPORTED')
    throw new Error('IDENTITY_MISMATCH_BLOCKS_EXECUTION');

  const snapshotArg = getValue(args, '--snapshot');
  const sourceArg = getValue(args, '--source');
  let sessionId: string | undefined;
  let startedAt: number | undefined;
  let sourceBeforeFingerprint: string | undefined;
  if (args.has('--run-review')) {
    if (!snapshotArg || !sourceArg) throw new Error('SNAPSHOT_AND_SOURCE_REQUIRED');
    const snapshot = await realpath(resolve(snapshotArg));
    const source = await realpath(resolve(sourceArg));
    if (matchesPath(source, snapshot) || matchesPath(snapshot, source))
      throw new Error('SNAPSHOT_MUST_BE_INDEPENDENT');
    if (snapshot.length > 4000) throw new Error('SNAPSHOT_PATH_TOO_LONG');
    const expectedHeadOID = getValue(args, '--expected-head');
    const expectedMergeBaseOID = getValue(args, '--expected-merge-base');
    if (!expectedHeadOID || !expectedMergeBaseOID) throw new Error('FROZEN_OID_ARGUMENTS_REQUIRED');
    inspectFrozenSnapshot({
      sourceRoot: source, snapshotRoot: snapshot, expectedHeadOID, expectedMergeBaseOID,
    });
    sourceBeforeFingerprint = originalRepositoryFingerprint(source);
    const locationKey = manifest.requestFields.sessionLocation;
    const nameKey = manifest.requestFields.commandName;
    const textKey = manifest.requestFields.commandText;
    for (const k of [locationKey, nameKey, textKey])
      if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(k)) throw new Error('UNVERIFIED_REQUEST_FIELD');
    const created = await client.call('createSession', 'POST', { [locationKey]: snapshot });
    sessionId = assertNativeId(field(created.body, manifest.responseFields.sessionId));
    record('session.create', created.status >= 200 && created.status < 300 ? 'SUPPORTED' : 'UNSUPPORTED',
      'Real native createSession HTTP ' + created.status + ' session captured (ID redacted)');
    const location = field(created.body, manifest.responseFields.sessionLocation);
    record('session.location', location === snapshot ? 'SUPPORTED' : 'UNKNOWN',
      location === snapshot ? 'Native session location matched separate snapshot' : 'Session location requires native follow-up inspection');
    if (created.status < 200 || created.status >= 300) throw new Error('SESSION_CREATE_NOT_ACCEPTED');
    startedAt = Date.now();
    const submitted = await client.call('command', 'POST',
      { [nameKey]: REVIEW_COMMAND, [textKey]: reviewArguments(getValue(args, '--controller')) },
      { sessionId });
    // HTTP 2xx/204 != native completion; never mark command.execute SUPPORTED here.
    record('command.execute', 'UNKNOWN',
      'Native command submit returned HTTP ' + submitted.status + '; still requires native terminal-state, Skill trace and report');
    if (submitted.status < 200 || submitted.status >= 300) throw new Error('NATIVE_COMMAND_SUBMIT_REJECTED');
  }

  if (args.has('--verify-report')) {
    if (!snapshotArg) throw new Error('SNAPSHOT_REQUIRED_FOR_REPORT');
    const value = getValue(args, '--run-started-at');
    if (!value || !/^\d+$/.test(value)) throw new Error('RUN_STARTED_AT_REQUIRED');
    const report = await verifyWrittenReport(await realpath(resolve(snapshotArg)), Number(value));
    const expectedBefore = getValue(args, '--before-fingerprint');
    if (sourceArg && expectedBefore) {
      if (!/^[0-9a-f]{64}$/.test(expectedBefore)) throw new Error('INVALID_SOURCE_FINGERPRINT');
      const observedAfter = originalRepositoryFingerprint(sourceArg);
      record('repository.immutable', observedAfter === expectedBefore ? 'SUPPORTED' : 'UNSUPPORTED',
        observedAfter === expectedBefore ? 'Original source fingerprint unchanged after native terminal run' :
          'Original source fingerprint CHANGED; fail isolation acceptance');
    }
    record('report.persist', 'SUPPORTED',
      'Native written five-chapter report verified: sha256=' + report.sha256 + ', bytes=' + report.bytes);
  }

  const result = evaluateCapabilities(observations);
  const document = {
    schema: 'code-harness-t01-native-probe-v1',
    runId, performedAt: new Date().toISOString(),
    environment: { platform: process.platform, node: process.version },
    build: manifest.build, serviceIdentity: manifest.serviceIdentity,
    sessionIdRedacted: sessionId ? 'captured' : null,
    startedAt, sourceBeforeFingerprint, capabilities: result.capabilities, blockers: result.blockers,
    ready: result.ready, notesEnabled: result.notesEnabled, continuity: result.continuity,
    caution: 'Partial transport checks only. Native permission/agent/skill/report/prompt/interrupt must be independently witnessed.',
  };
  await mkdir(dirname(outPath), { recursive: true });
  await writeFile(outPath, JSON.stringify(document, null, 2) + '\n', { encoding: 'utf8', mode: 0o600 });
  console.log('T01 result: ' + outPath + ' | ready=' + result.ready +
    ' | blocking=' + result.blockers.length);
  if (!result.ready) process.exitCode = 2;
}

main().catch(error => {
  // Do not leak raw HTTP payloads, credentials, source paths or stack traces.
  console.error('T01 probe blocked: ' + (error instanceof Error ? error.message : 'UNKNOWN_ERROR'));
  process.exitCode = 2;
});
