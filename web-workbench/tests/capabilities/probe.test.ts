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
