import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { CAPABILITIES, evaluateCapabilities, requireReady, type Observation } from '../../packages/opencode-adapter/src/probe.ts';
import { reviewArguments } from '../../packages/opencode-adapter/src/mapping.ts';

type CapabilityName = (typeof CAPABILITIES)[number]['name'];
const verified = (): Partial<Record<CapabilityName, Observation>> => Object.fromEntries(CAPABILITIES.map(x =>
  [x.name, { status: 'SUPPORTED', evidence: 'windows-native://run-1/' + x.name, windowsRunId: 'run-1' }]
)) as Partial<Record<CapabilityName, Observation>>;

describe('T01 strict capability gate', () => {
  it('does not declare readiness without company Windows evidence', () => {
    const result = evaluateCapabilities();
    assert.equal(result.ready, false);
    assert.ok(result.capabilities.every(x => x.status === 'UNKNOWN'));
    assert.throws(() => requireReady(result), /OPENCODE_CAPABILITY_BLOCKED/);
  });
  it('missing command, trusted skill or native permission blocks execution', () => {
    for (const name of ['command.execute', 'command.trusted', 'skill.trusted', 'permission.native'] as const) {
      const data = verified();
      delete data[name];
      const result = evaluateCapabilities(data);
      assert.equal(result.ready, false, name);
      assert.ok(result.blockers.includes(name), name);
    }
  });
  it('missing instruction entry disables notes but durable log absence only degrades continuity', () => {
    const data = verified();
    delete data['instruction.entry'];
    delete data['session.log'];
    const result = evaluateCapabilities(data);
    assert.equal(result.ready, true);
    assert.equal(result.notesEnabled, false);
    assert.equal(result.continuity, 'GAP');
  });
  it('private model unavailable blocks readiness and evidence-only claims stay UNKNOWN', () => {
    const data = verified();
    data['model.private'] = { status: 'UNSUPPORTED', evidence: 'Native run-1 model connection failed', windowsRunId: 'run-1' };
    assert.equal(evaluateCapabilities(data).ready, false);
    data['model.private'] = { status: 'SUPPORTED', evidence: 'self assertion without real trace' };
    assert.equal(evaluateCapabilities(data).capabilities.find(x => x.name === 'model.private')?.status, 'UNKNOWN');
  });
  it('only accepts safe fixed-base command arguments', () => {
    assert.equal(reviewArguments(), '--base harness-review-base');
    assert.equal(reviewArguments('OrderController'), '--base harness-review-base --controller OrderController');
    for (const val of ['Order;rm -rf', '订单', 'A B', 'A$HOME', 'A'.repeat(129), '']) {
      assert.throws(() => reviewArguments(val), /INVALID_CONTROLLER/);
    }
  });
});

import { LocalNativeTransport } from '../../packages/opencode-adapter/src/native.ts';

describe('T01 wire transport boundaries', () => {
  it('rejects any non-loopback or unverified native service', () => {
    for (const baseUrl of ['https://example.com', 'http://192.168.1.8:4096', 'http://127.0.0.1:4096@evil.example']) {
      assert.throws(() => new LocalNativeTransport({
        baseUrl, build: 'company-build-verified', serviceIdentity: 'company-service', paths: {},
      }), /NATIVE_ENDPOINT_NOT_LOOPBACK/);
    }
  });
  it('rejects unknown native route shapes rather than guessing endpoints', async () => {
    const client = new LocalNativeTransport({
      baseUrl: 'http://127.0.0.1:4096', build: 'company-build', serviceIdentity: 'company-service', paths: {},
    });
    await assert.rejects(() => client.call('permissions', 'GET'), /NATIVE_ROUTE_NOT_VERIFIED/);
  });
});


import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import { inspectFrozenSnapshot, originalRepositoryFingerprint } from '../../packages/opencode-adapter/src/git-guard.ts';

describe('T01 fixed committed-only Git task location', () => {
  it('allows independent clean clone with pinned OIDs and rejects subsequent dirty snapshot', () => {
    const parent = mkdtempSync(join(tmpdir(), 'harness-t01-'));
    const source = join(parent, 'original');
    const snapshot = join(parent, 'task-copy');
    function run(args: string[]): string {
      return execFileSync('git', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
    }
    try {
      mkdirSync(source);
      run(['init', '-q', source]);
      writeFileSync(join(source, 'hello.txt'), 'hello\n');
      run(['-C', source, 'add', 'hello.txt']);
      run(['-C', source, '-c', 'user.name=T01', '-c', 'user.email=t01@example.invalid', 'commit', '-qm', 'initial']);
      const head = run(['-C', source, 'rev-parse', 'HEAD']);
      run(['clone', '-q', '--no-hardlinks', source, snapshot]);
      run(['-C', snapshot, 'branch', 'harness-review-base', head]);
      const before = originalRepositoryFingerprint(source);
      const frozen = inspectFrozenSnapshot({
        sourceRoot: source, snapshotRoot: snapshot, expectedHeadOID: head, expectedMergeBaseOID: head,
      });
      assert.equal(frozen.headOID, head);
      assert.equal(frozen.baseOID, head);
      writeFileSync(join(snapshot, 'extra.sql'), 'SELECT 1;\n');
      assert.throws(() => inspectFrozenSnapshot({
        sourceRoot: source, snapshotRoot: snapshot, expectedHeadOID: head, expectedMergeBaseOID: head,
      }), /SNAPSHOT_NOT_CLEAN/);
      assert.equal(originalRepositoryFingerprint(source), before);
      assert.throws(() => inspectFrozenSnapshot({
        sourceRoot: source, snapshotRoot: snapshot, expectedHeadOID: '0'.repeat(40), expectedMergeBaseOID: head,
      }), /FROZEN_GIT_SCOPE_MISMATCH/);
    } finally {
      rmSync(parent, { recursive: true, force: true });
    }
  });
});


import { verifyWrittenReport } from '../../packages/opencode-adapter/src/native.ts';

describe('T01 native report file proof', () => {
  it('requires five ordered chapters, fresh physical file and digest', async () => {
    const root = mkdtempSync(join(tmpdir(), 'harness-report-'));
    try {
      mkdirSync(join(root, 'temp'));
      const file = join(root, 'temp', 'codea-review-report.md');
      const markdown = [
        '# 代码评审报告', '## 评审摘要', '## 调用链与覆盖范围',
        '## 风险清单', '## 问题明细', '## 后续处理与评审边界',
      ].join('\n');
      const started = Date.now() - 2000;
      writeFileSync(file, markdown);
      const result = await verifyWrittenReport(root, started);
      assert.equal(result.sections, 5);
      assert.equal(result.sha256.length, 64);
      await assert.rejects(() => verifyWrittenReport(root, Date.now() + 30000),
        /REPORT_MISSING_OR_STALE/);
      writeFileSync(file, '# Only heading\n');
      await assert.rejects(() => verifyWrittenReport(root, started),
        /REPORT_FIVE_CHAPTERS_INVALID/);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });
});
