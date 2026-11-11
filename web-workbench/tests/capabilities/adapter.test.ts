import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';
import type { AddressInfo } from 'node:net';
import type { Snapshot } from '../../packages/contracts/src/domain.ts';
import { LocalNativeTransport, type NativeWireManifest } from '../../packages/opencode-adapter/src/native.ts';
import { HttpOpenCodeAdapter } from '../../packages/opencode-adapter/src/adapter.ts';
import { CAPABILITIES, type Observation } from '../../packages/opencode-adapter/src/probe.ts';

const paths: NativeWireManifest['paths'] = {
  createSession: '/api/session', session: '/api/session/{sessionID}',
  command: '/api/session/{sessionID}/command',
  prompt: '/api/session/{sessionID}/prompt',
  active: '/api/session/active',
  inbox: '/api/session/{sessionID}/inbox',
  messages: '/api/session/{sessionID}/messages',
  permissions: '/api/session/{sessionID}/permission',
  permissionReply: '/api/session/{sessionID}/permission/{requestID}/reply',
  forms: '/api/session/{sessionID}/form',
  formReply: '/api/session/{sessionID}/form/{formID}/reply',
  interrupt: '/api/session/{sessionID}/interrupt',
  events: '/api/event',
};
const evidence = (): Partial<Record<(typeof CAPABILITIES)[number]['name'], Observation>> =>
  Object.fromEntries(CAPABILITIES.map(cap => [cap.name, {
    status: 'SUPPORTED', evidence: 'MOCK_ONLY_NEVER_USE_IN_PRODUCTION', windowsRunId: 'fixture-only',
  }])) as Partial<Record<(typeof CAPABILITIES)[number]['name'], Observation>>;

describe('T01 Adapter local HTTP harness: not company native evidence', () => {
  it('blocks actions with UNKNOWN production readiness', async () => {
    const adapter = new HttpOpenCodeAdapter({
      transport: new LocalNativeTransport({
        baseUrl: 'http://127.0.0.1:4096', build: 'fixture', serviceIdentity: 'fixture', paths,
      }),
      verifySnapshot: async () => {},
      mapInteraction: () => { throw new Error('NOT_MAPPED'); },
      getNativeRequestId: () => { throw new Error('NOT_MAPPED'); },
    });
    await assert.rejects(() => adapter.createSession(randomUUID(), {
      taskId: randomUUID(), storageKey: '/tmp', manifestHash: '', scope: {} as Snapshot['scope'],
    }), /OPENCODE_CAPABILITY_BLOCKED/);
  });

  it('creates pinned Session, sends command once, handles permission, prompt, interrupt and messages', async () => {
    const requests: Array<{ method: string; path: string; body: unknown }> = [];
    const root = mkdtempSync(join(tmpdir(), 'harness-adapter-'));
    const sessionId = 'ses-native-test-1';
    let alreadyAnswered = false;
    const server = createServer(async (req, res) => {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(chunk as Buffer);
      const raw = Buffer.concat(chunks).toString('utf8');
      const body: unknown = raw ? JSON.parse(raw) as unknown : undefined;
      const path = new URL(req.url!, 'http://127.0.0.1').pathname;
      requests.push({ method: req.method!, path, body });
      res.setHeader('Content-Type', 'application/json');
      if (path === '/api/event') {
        res.setHeader('Content-Type', 'text/event-stream');
        for (const id of ['1', '2']) {
          res.write('id: ' + id + '\ndata: ' +
            JSON.stringify({ sessionID: sessionId, type: 'fixture.progress' }) + '\n\n');
        }
        res.end();
        return;
      }
      if (path === '/api/session' && req.method === 'POST') {
        res.end(JSON.stringify({ data: { id: sessionId, agent: 'codea-reviewer',
          location: (body as { location: { directory: string } }).location } })); return;
      }
      if (path.endsWith('/command')) { res.statusCode = 204; res.end(); return; }
      if (path.endsWith('/prompt')) {
        res.end(JSON.stringify({ data: { id: 'native-prompt-1' } })); return;
      }
      if (path === '/api/session/active') {
        res.end(JSON.stringify({ data: { [sessionId]: { status: 'running' } } })); return;
      }
      if (path.endsWith('/inbox')) { res.end(JSON.stringify({ data: [] })); return; }
      if (path.endsWith('/messages')) {
        res.end(JSON.stringify({ data: [{ id: 'native-msg', role: 'assistant',
          text: 'Review completed', status: 'COMPLETE',
          createdAt: '2026-11-03T00:00:00Z', updatedAt: '2026-11-03T00:00:01Z' }] })); return;
      }
      if (path.endsWith('/permission') && req.method === 'GET') {
        res.end(JSON.stringify({ data: alreadyAnswered ? [] :
          [{ id: 'perm-native-1', sessionID: sessionId, action: 'shell',
            resources: ['git diff'] }] })); return;
      }
      if (path.endsWith('/form') && req.method === 'GET') {
        res.end(JSON.stringify({ data: [] })); return;
      }
      if (path.includes('/permission/') && path.endsWith('/reply')) {
        alreadyAnswered = true; res.statusCode = 204; res.end(); return;
      }
      if (path.endsWith('/interrupt')) {
        res.end(JSON.stringify({ data: { interrupted: true } })); return;
      }
      res.statusCode = 404; res.end(JSON.stringify({ error: 'not found' }));
    });
    try {
      await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
      const port = (server.address() as AddressInfo).port;
      const adapter = new HttpOpenCodeAdapter({
        transport: new LocalNativeTransport({
          baseUrl: 'http://127.0.0.1:' + port,
          build: 'company-test-only', serviceIdentity: 'local-test-server', paths,
        }),
        observations: evidence(),
        verifySnapshot: async snapshot => {
          assert.equal(snapshot.storageKey, root);
          assert.equal(snapshot.manifestHash, 'sha256-fixture');
        },
        getNativeRequestId: raw => (raw as { id: string }).id,
        mapInteraction: (raw, kind, context) => ({
          id: context.businessId, taskId: context.taskId, runId: context.runId,
          kind: 'PERMISSION', revision: 1, status: 'PENDING',
          expiresAt: '2030-01-01T00:00:00Z', tool: (raw as { action: string }).action,
          requestText: 'git diff', allowedDecisions: ['ALLOW_ONCE', 'DENY'],
        }),
      });
      const taskId = randomUUID();
      const snapshot: Snapshot = {
        taskId, storageKey: root, manifestHash: 'sha256-fixture',
        scope: {} as Snapshot['scope'],
      };
      const bound = await adapter.createSession(taskId, snapshot);
      assert.equal(bound.sessionId, sessionId);
      assert.equal(bound.serviceIdentity, 'local-test-server');
      const runId = randomUUID();
      const accepted = await adapter.startReview(bound, { runId, controller: 'OrderController' });
      assert.equal(accepted.state, 'UNKNOWN'); // command 204 is NOT completed / accepted model
      await assert.rejects(() => adapter.startReview(bound, { runId }), /SESSION_RUN_ALREADY_DISPATCHED/);
      assert.equal(requests.filter(r => r.path.endsWith('/command')).length, 1);
      const sent = requests.find(r => r.path.endsWith('/command'))!.body as { name: string; text: string };
      assert.deepEqual(sent, { name: 'codea-review',
        text: '--base harness-review-base --controller OrderController' });
      const pending = await adapter.listInteractions(bound);
      assert.equal(pending.length, 1);
      assert.equal(pending[0].kind, 'PERMISSION');
      const changed=await adapter.answer(bound,pending[0].id,{kind:'PERMISSION',decision:'DENY'},
        {revision:2,nativeRequestId:'perm-native-1',schema:{}});
      assert.equal(changed.state,'REJECTED');
      assert.equal(requests.filter(r=>r.path.endsWith('/reply')).length,0);
      const reply = await adapter.answer(bound, pending[0].id,
        { kind: 'PERMISSION', decision: 'DENY' });
      assert.equal(reply.state, 'UNKNOWN');
      const wire = requests.find(r => r.path.includes('/permission/') && r.path.endsWith('/reply'))!;
      assert.deepEqual(wire.body, { decision: 'reject' });
      assert.equal((await adapter.answer(bound, pending[0].id,
        { kind: 'PERMISSION', decision: 'ALLOW_ONCE' })).state, 'REJECTED');
      const messages = await adapter.readMessages(bound);
      assert.equal(messages.items[0].role, 'ASSISTANT');
      assert.equal(messages.items[0].status, 'COMPLETE');
      assert.equal(messages.items[0].runId, runId);
      const inspected = await adapter.inspect(bound);
      assert.equal(inspected.active, true);
      assert.equal(inspected.pendingInteractions, 0);
      const stream = adapter.subscribe(bound)[Symbol.asyncIterator]();
      const first = await stream.next();
      assert.equal(first.done, false);
      assert.equal(first.value.runId, runId);
      assert.equal(first.value.cursor, '1');
      const prompt = await adapter.discuss(bound, {
        runId: randomUUID(), messageId: randomUUID(), text: 'What is the SQL risk?',
      });
      await assert.rejects(stream.next(), /NATIVE_EVENT_RUN_CHANGED/);
      assert.equal(prompt.state, 'ACCEPTED');
      assert.equal(prompt.inputId, 'native-prompt-1');
      assert.deepEqual(await adapter.interrupt(bound), { responseReceived: true, interrupted: true });
      const snapshotTraces = requests.filter(r => r.path.endsWith('/command'));
      assert.equal(snapshotTraces.length, 1);
    } finally {
      await new Promise<void>((resolve, reject) =>
        server.close(error => error ? reject(error) : resolve()));
      rmSync(root, { force: true, recursive: true });
    }
  });
});
