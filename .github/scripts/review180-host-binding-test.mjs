import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import { createHash } from 'node:crypto'

const { default: tool } = await import(pathToFileURL(process.env.CODEA_REVIEW_TEST_TOOL).href)
const exact = 'review-2de94b7844a525056e2153033cdfc25c'
const typo = 'review-2de94b7844a525056e3032153cdfc25c'
const sha = value => createHash('sha256').update(value).digest('hex')

async function fixture(t) {
  const root = await mkdtemp(path.join(tmpdir(), 'codea binding 空格 & # % '))
  t.after(() => rm(root, { recursive: true, force: true }))
  const dir = path.join(root, '.code-harness', 'runs', exact)
  await mkdir(dir, { recursive: true })
  const source = 'class OrderController {}\n'
  const ref = { path: 'OrderController.java', sha256: sha(source), startLine: 1, endLine: 1 }
  await writeFile(path.join(root, ref.path), source)
  const reportPath = path.join(dir, 'review.md')
  await writeFile(reportPath, 'INCOMPLETE\n')
  const receipt = { runId: exact, execution: 'INCOMPLETE', reviewConclusion: 'UNDETERMINED', coverage: 'PARTIAL', reportPath, reportSha256: sha('INCOMPLETE\n') }
  await writeFile(path.join(dir, 'run.json'), JSON.stringify({ schemaVersion: 1, runId: exact, reportPath }))
  await writeFile(path.join(dir, 'scope.json'), JSON.stringify({ runId: exact, selectedIds: ['C1'], reads: [ref], findingReads: [ref] }))
  const text = JSON.stringify(receipt, null, 2) + '\r\n\r\n# Codea Harness 1.8 Review\r\n\r\nReview target: \r\n'
  const message = { info: { role: 'user', id: 'msg_entry', sessionID: 'ses_current' }, parts: [{ type: 'text', text, messageID: 'msg_entry', sessionID: 'ses_current' }] }
  const context = { worktree: root, directory: root, sessionID: 'ses_current', messageID: 'msg_assistant', agent: 'codea-review-restricted', messages: [message] }
  return { root, dir, ref, receipt, message, context, args: { action: 'read', source: ref } }
}

test('CI205: read uses exact Host entry without model transcription', async t => {
  const f = await fixture(t)
  const out = JSON.parse(await tool.execute(f.args, f.context))
  assert.equal(out.source.lines[0].text, 'class OrderController {}')
  assert.deepEqual(out.hostBinding, { runId: exact, sessionID: 'ses_current', userMessageID: 'msg_entry', toolMessageID: 'msg_assistant' })
})

test('CI205: mistranscribed model ID cannot override Host entry', async t => {
  const f = await fixture(t)
  await assert.rejects(tool.execute({ ...f.args, runId: typo }, f.context), /CODEA_REVIEW_RUN_ID_MISMATCH/)
})

test('another existing run cannot override Host entry', async t => {
  const f = await fixture(t)
  await mkdir(path.join(f.root, '.code-harness', 'runs', typo))
  await writeFile(path.join(f.root, '.code-harness', 'runs', typo, 'run.json'), JSON.stringify({ runId: typo }))
  await assert.rejects(tool.execute({ ...f.args, runId: typo }, f.context), /CODEA_REVIEW_RUN_ID_MISMATCH/)
})

test('missing native messages never falls back to model ID or directories', async t => {
  const f = await fixture(t)
  delete f.context.messages
  await assert.rejects(tool.execute({ ...f.args, runId: exact }, f.context), /CODEA_REVIEW_HOST_ENTRY_REQUIRED/)
})

test('a new user turn cannot reuse an older command receipt', async t => {
  const f = await fixture(t)
  f.context.messages.push({ info: { role: 'user', id: 'msg_followup', sessionID: 'ses_current' }, parts: [{ type: 'text', text: 'continue', messageID: 'msg_followup', sessionID: 'ses_current' }] })
  await assert.rejects(tool.execute({ ...f.args, runId: exact }, f.context), /CODEA_REVIEW_HOST_ENTRY_REQUIRED/)
})

test('cross-session receipt rejected', async t => {
  const f = await fixture(t)
  f.context.sessionID = 'ses_other'
  await assert.rejects(tool.execute(f.args, f.context), /CODEA_REVIEW_HOST_ENTRY/)
})

test('duplicate entry receipts rejected', async t => {
  const f = await fixture(t)
  f.message.parts.push({ ...f.message.parts[0] })
  await assert.rejects(tool.execute(f.args, f.context), /CODEA_REVIEW_HOST_ENTRY/)
})

test('receipt path cannot point at a different run', async t => {
  const f = await fixture(t)
  f.message.parts[0].text = f.message.parts[0].text.replaceAll(f.receipt.reportPath.split('\\').join('\\\\'), path.join(f.root, 'review.md').split('\\').join('\\\\'))
  await assert.rejects(tool.execute(f.args, f.context), /CODEA_REVIEW_HOST_ENTRY/)
})

test('missing run state fails closed despite an authorized-looking scope', async t => {
  const f = await fixture(t)
  await rm(path.join(f.dir, 'run.json'))
  await assert.rejects(tool.execute(f.args, f.context), /CODEA_REVIEW_HOST_ENTRY/)
})

test('run state identity mismatch rejected', async t => {
  const f = await fixture(t)
  await writeFile(path.join(f.dir, 'run.json'), JSON.stringify({ runId: typo, reportPath: f.receipt.reportPath }))
  await assert.rejects(tool.execute(f.args, f.context), /CODEA_REVIEW_HOST_ENTRY/)
})

test('prepare checks initial report hash before invoking Runtime', async t => {
  const f = await fixture(t)
  await writeFile(f.receipt.reportPath, 'tampered')
  await assert.rejects(tool.execute({ action: 'prepare', intent: { mode: 'CHANGES' } }, f.context), /CODEA_REVIEW_HOST_ENTRY/)
})

test('source hash verification remains mandatory after Host binding', async t => {
  const f = await fixture(t)
  await writeFile(path.join(f.root, f.ref.path), 'changed source\n')
  await assert.rejects(tool.execute(f.args, f.context), /CODEA_REVIEW_SOURCE_STALE/)
})

test('Host binding never widens the authorized source tuple', async t => {
  const f = await fixture(t)
  await assert.rejects(tool.execute({ ...f.args, source: { ...f.ref, endLine: 2 } }, f.context), /CODEA_REVIEW_SOURCE_OUTSIDE_SCOPE/)
})

test('a matching legacy ID still needs the Host receipt', async t => {
  const f = await fixture(t)
  const out = JSON.parse(await tool.execute({ ...f.args, runId: exact }, f.context))
  assert.equal(out.hostBinding.runId, exact)
})
