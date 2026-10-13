import { tool } from "@opencode-ai/plugin"
import { execFile } from "node:child_process"
import { createHash, randomUUID } from "node:crypto"
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises"
import path from "node:path"
import { promisify } from "node:util"

const execFileAsync = promisify(execFile)
const runIDPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/

type HostMessage180 = {
  info: { role: string; id: string; sessionID: string }
  parts: { type: string; text?: string; messageID?: string; sessionID?: string }[]
}

// OpenCode 1.18.25's native registry forwards Tool.Context.messages to custom
// tools. Consume that Host-owned context, never model-copied run identifiers.
// If a different Host omits it, fail closed rather than selecting a recent run.
async function hostRunBinding(worktree: string, context: {
  sessionID: string; messageID: string; messages?: HostMessage180[]
}, action: string) {
  const messages = context.messages
  if (!context.sessionID || !context.messageID || !Array.isArray(messages)) {
    throw new Error("CODEA_REVIEW_HOST_ENTRY_REQUIRED")
  }
  const user = messages.findLast(message => message.info?.role === "user")
  if (!user?.info.id || user.info.sessionID !== context.sessionID || !Array.isArray(user.parts)) {
    throw new Error("CODEA_REVIEW_HOST_ENTRY_REQUIRED")
  }
  // Only the current user turn's fixed command expansion may bind this run.
  // Do not search older messages, model reasoning, tool arguments or directories.
  const receipts = user.parts.flatMap(part => {
    if (part.type !== "text" || typeof part.text !== "string") return []
    const match = /^\s*(\{[\s\S]*?\r?\n\})\s*# Codea Harness 1\.8 Review\r?\n/.exec(part.text)
    if (!match) return []
    if (part.sessionID !== context.sessionID || part.messageID !== user.info.id) {
      throw new Error("CODEA_REVIEW_HOST_ENTRY_IDENTITY_INVALID")
    }
    try { return [JSON.parse(match[1])] } catch {
      throw new Error("CODEA_REVIEW_HOST_ENTRY_INVALID")
    }
  })
  if (receipts.length !== 1) throw new Error("CODEA_REVIEW_HOST_ENTRY_REQUIRED")
  const receipt = receipts[0]
  if (typeof receipt.runId !== "string" || !runIDPattern.test(receipt.runId) ||
      receipt.execution !== "INCOMPLETE" || receipt.reviewConclusion !== "UNDETERMINED" ||
      receipt.coverage !== "PARTIAL" || typeof receipt.reportPath !== "string" ||
      !/^[a-f0-9]{64}$/.test(receipt.reportSha256 ?? "")) {
    throw new Error("CODEA_REVIEW_HOST_ENTRY_INVALID")
  }
  const runDir = path.resolve(worktree, ".code-harness", "runs", receipt.runId)
  const reportPath = path.join(runDir, "review.md")
  if (!path.isAbsolute(receipt.reportPath) || path.relative(reportPath, receipt.reportPath) !== "") {
    throw new Error("CODEA_REVIEW_HOST_ENTRY_PATH_MISMATCH")
  }
  try {
    const rootReal = await realpath(worktree)
    const expectedDir = path.join(rootReal, ".code-harness", "runs", receipt.runId)
    if (path.relative(expectedDir, await realpath(runDir)) !== "" ||
        path.relative(path.join(expectedDir, "run.json"), await realpath(path.join(runDir, "run.json"))) !== "" ||
        path.relative(path.join(expectedDir, "review.md"), await realpath(reportPath)) !== "") {
      throw new Error("run state or report redirected")
    }
    const state = JSON.parse(await readFile(path.join(runDir, "run.json"), "utf8"))
    if (state.runId !== receipt.runId || state.reportPath !== receipt.reportPath) {
      throw new Error("run state identity mismatch")
    }
    // prepare must consume the exact durable INCOMPLETE report read back by
    // start. Later actions intentionally see Runtime's updated report bytes.
    if (action === "prepare") {
      const digest = createHash("sha256").update(await readFile(reportPath)).digest("hex")
      if (digest !== receipt.reportSha256) throw new Error("entry report hash mismatch")
    }
  } catch (error) {
    throw new Error(`CODEA_REVIEW_HOST_ENTRY_STATE_INVALID: ${String(error)}`)
  }
  return { runId: receipt.runId as string, sessionID: context.sessionID,
    userMessageID: user.info.id, toolMessageID: context.messageID }
}

const readRef = tool.schema.object({
  path: tool.schema.string(),
  sha256: tool.schema.string(),
  startLine: tool.schema.number().int().positive(),
  endLine: tool.schema.number().int().positive(),
})
const evidence = tool.schema.object({
  ref: readRef,
  quote: tool.schema.string(),
})
const finding = tool.schema.object({
  id: tool.schema.string(),
  severity: tool.schema.enum(["CRITICAL", "HIGH", "MEDIUM", "LOW"]),
  problem: tool.schema.string(),
  impact: tool.schema.string(),
  recommendation: tool.schema.string(),
  verification: tool.schema.string(),
  evidence: tool.schema.array(evidence),
  introducedByChange: tool.schema.boolean().optional(),
})

async function exclusiveWrite(target: string, data: string) {
  await mkdir(path.dirname(target), { recursive: true })
  await writeFile(target, data, { encoding: "utf8", mode: 0o644, flag: "wx" })
}

function runtimePath(worktree: string) {
  const name = process.platform === "win32" ? "codea-dcep-tools.exe" : "codea-dcep-tools"
  return path.resolve(worktree, ".code-harness", "bin", name)
}

async function invoke(worktree: string, argv: string[]) {
  try {
    const { stdout } = await execFileAsync(runtimePath(worktree), argv, {
      cwd: worktree,
      windowsHide: true,
      timeout: 125_000,
      maxBuffer: 4 * 1024 * 1024,
    })
    const text = stdout.trim()
    if (!text) throw new Error("Runtime returned empty output")
    return JSON.parse(text) as Record<string, unknown>
  } catch (error) {
    throw new Error(`CODEA_REVIEW_RUNTIME_FAILED: ${String(error)}`)
  }
}

async function readScope(worktree: string, runId: string) {
  const scopePath = path.resolve(worktree, ".code-harness", "runs", runId, "scope.json")
  try {
    return JSON.parse(await readFile(scopePath, "utf8")) as Record<string, unknown>
  } catch {
    return undefined
  }
}


type SourceRef180 = { path: string; sha256: string; startLine: number; endLine: number }

// Only explicitly prepared source tuples are readable. No arbitrary file reads,
// directory enumeration, symlinks outside worktree, or unchecked stale source.
async function readAuthorizedSource(worktree: string, runId: string, requested: SourceRef180) {
  const scope = await readScope(worktree, runId)
  if (!scope || scope.runId !== runId || !Array.isArray(scope.reads) ||
      !Array.isArray(scope.selectedIds) || scope.selectedIds.length === 0) {
    throw new Error("CODEA_REVIEW_SOURCE_SCOPE_NOT_READY")
  }
  const safe = requested.path
  if (!safe || safe.includes("\\") || safe.includes("\0") || safe.includes(":") ||
      safe.startsWith("/") || safe.split("/").some(part => !part || part === "." || part === "..")) {
    throw new Error("CODEA_REVIEW_SOURCE_PATH_INVALID")
  }
  const declared = (scope.reads as SourceRef180[]).some(ref =>
    ref.path === requested.path &&
    ref.sha256 === requested.sha256 &&
    ref.startLine === requested.startLine &&
    ref.endLine === requested.endLine
  )
  if (!declared) throw new Error("CODEA_REVIEW_SOURCE_OUTSIDE_SCOPE")
  const rootReal = await realpath(worktree)
  const sourceReal = await realpath(path.join(worktree, ...safe.split("/")))
  const relative = path.relative(rootReal, sourceReal)
  if (!relative || relative === ".." || relative.startsWith(".." + path.sep) || path.isAbsolute(relative)) {
    throw new Error("CODEA_REVIEW_SOURCE_PATH_INVALID")
  }
  const bytes = await readFile(sourceReal)
  const digest = createHash("sha256").update(bytes).digest("hex")
  if (digest !== requested.sha256) throw new Error("CODEA_REVIEW_SOURCE_STALE")
  const lines = bytes.toString("utf8").split("\n").map(line => line.endsWith("\r") ? line.slice(0, -1) : line)
  if (requested.startLine < 1 || requested.endLine < requested.startLine || requested.endLine > lines.length) {
    throw new Error("CODEA_REVIEW_SOURCE_RANGE_INVALID")
  }
  // scope.reads permits source CONTEXT; scope.findingReads permits EVIDENCE.
  // Never advertise an import, class header or sibling method as quotable
  // just because the entire file is authorized for contextual reading.
  if (!Array.isArray(scope.findingReads)) throw new Error("CODEA_REVIEW_FINDING_SCOPE_NOT_READY")
  const findingRanges = (scope.findingReads as SourceRef180[]).filter(ref =>
    ref.path === requested.path && ref.sha256 === requested.sha256 &&
    Number.isInteger(ref.startLine) && Number.isInteger(ref.endLine) &&
    ref.startLine >= 1 && ref.endLine >= ref.startLine && ref.endLine <= lines.length
  )
  if (findingRanges.length === 0) throw new Error("CODEA_REVIEW_FINDING_SCOPE_NOT_READY")
  const annotatedLines = lines.slice(requested.startLine - 1, requested.endLine)
    .map((text, offset) => {
      const line = requested.startLine + offset
      return {
        line, text,
        evidenceEligible: findingRanges.some(ref => ref.startLine <= line && line <= ref.endLine),
      }
    })
  return {
    source: {
      ref: requested,
      findingRanges,
      lines: annotatedLines,
    },
    nextAction: "Only source.lines with evidenceEligible=true may be quoted in finding.evidence; evidenceEligible=false lines are CONTEXT ONLY, never evidence. Only cite an exact physical source line within scope.findingReads and preserve the real SHA/path/line. The Runtime independently checks exact code spans and will reject even a marked line if its quoted bytes fall outside the method. Continue reading all prepared scope.reads and finish once in this same assistant turn.",
  }
}

function nextAction(runtime: Record<string, unknown>, scope?: Record<string, unknown>) {
  if (scope) {
    return {
      type: "READ_SCOPE_AND_FINISH_THIS_TURN",
      mandatory: true,
      assistantTurnTerminal: false,
      mustContinueToolExecution: true,
      finishRequiredEvenWhenFindingsEmpty: true,
      instruction: "For every source read call only codea-review action=read with an exact ref from scope.reads (path, sha256, startLine, endLine). Never use built-in read, glob, grep, list, bash or out-of-scope exploration. Review the returned source, then call codea-review finish in this same assistant turn. Do not end the turn after prepare/select. findings=[] still requires finish. CRITICAL METHOD EVIDENCE SCOPE: scope.reads authorizes whole-file context only; scope.findingReads controls which method or SQL-statement lines may be cited. Every codea-review read response marks each source.lines item evidenceEligible=true/false. NEVER cite a line marked false (imports, class headers, unrelated sibling methods); select a true line showing the affected method/statement instead. Treat true as a necessary but not sufficient condition, because Runtime also validates exact code spans. Evidence quote rule: use a verbatim substring from a single physical source line for each evidence entry; set ref.startLine = ref.endLine to that exact line number, and include that read ref if necessary. For a defect spanning multiple lines, give several separate one-line evidence entries. Never dedent, reindent, splice, or compact multiline source into quote. If the Runtime reports REVIEW_FINISH_EVIDENCE_QUOTE_MISMATCH, reread that exact authorized line and replace the quote rather than merely changing ref.startLine/endLine. introducedByChange=true requires proof that the finding itself was introduced by changed Git lines. A finding in a change-impacted Controller chain does NOT imply its own defect was introduced by the change. For unchanged Controller authorization or tenant-boundary defects, use introducedByChange=false even if a downstream SQL statement changed. Never mark true merely because CHANGES mode selected the Controller; when change attribution cannot be proven, omit introducedByChange or set it false; do not submit an unproven true and then retry finish. pendingRisks must describe only a current unresolved harmful condition supported by current source whose confirmation needs evidence outside the authorized scope; do not report hypothetical future edits, future endpoint repurposing, or generic best-practice concerns.",
    }
  }
  return {
    type: "KEEP_REPORT_INCOMPLETE",
    mandatory: true,
    instruction: "No review scope is ready. Report the concrete prepare/select state and keep the durable report INCOMPLETE; do not manufacture findings or call finish without an authorized scope.",
  }
}

export default tool({
  description: "Codea Harness 1.8 ordinary review. The Host binds the exact run from the current command entry; omit runId from every call. Prepare, scope-verified read, finish. Use action=read with exact source ref from scope.reads; built-in read/glob/grep/list/bash are forbidden. Prepare automatically authorizes all change-impacted Controllers if no target is supplied, or only the explicitly targeted Controller if given, preserving method-level impact and scoped evidence. No human C1/C2 selection or auxiliary menu resume. If a scope is ready, review and finish in this same assistant turn, even when findings are empty. pendingRisks are only current unresolved harmful conditions supported by current source and requiring out-of-scope confirmation, never hypothetical future changes. Only source.lines with evidenceEligible=true (from scope.findingReads) can support a finding; false-marked context lines must NEVER become finding evidence. Each evidence.quote must be a verbatim substring of a single physical source line, cited with startLine=endLine; multiple evidence entries are preferable to a reformatted multiline block. introducedByChange=true requires proof that the finding itself was introduced by changed Git lines. A finding in a change-impacted Controller chain does NOT imply its own defect was introduced by the change. For unchanged Controller authorization or tenant-boundary defects, use introducedByChange=false even if a downstream SQL statement changed. Never mark true merely because CHANGES mode selected the Controller; when change attribution cannot be proven, omit introducedByChange or set it false; do not submit an unproven true and then retry finish. CHANGES is fail-closed: never retry or silently fall back to CURRENT_IMPLEMENTATION after no relevant changes or any CHANGES prepare failure.",
  args: {
    action: tool.schema.enum(["prepare", "read", "finish"]),
    source: readRef.optional(),
    intent: tool.schema.object({
      mode: tool.schema.enum(["CHANGES", "CURRENT_IMPLEMENTATION"]),
      target: tool.schema.string().optional(),
    }).optional(),
    result: tool.schema.object({
      reads: tool.schema.array(readRef),
      findings: tool.schema.array(finding),
      pendingRisks: tool.schema.array(tool.schema.string()),
      gaps: tool.schema.array(tool.schema.string()).describe("Specific information or evidence that is actually missing and prevents resolving a current review question. This is NOT a notes, rationale, or review-summary field. Do not put explanations that code is safe, intentional public access, absence of exploitable conditions, or hypothetical future concerns here. If authorized sources fully resolve the review and no information is missing, send gaps=[]. Put any no-issue rationale in the final natural-language response after successful finish. Preserve genuine missing-evidence gaps; never hide one to obtain a clean result."),
    }).optional(),
  },
  async execute(args, context) {
    const worktree = path.resolve(context.worktree)
    const hostBinding = await hostRunBinding(worktree, context, args.action)
    const runId = hostBinding.runId
    // Legacy/extra model fields must never redirect the Host-bound run.
    const suppliedRunId = (args as typeof args & { runId?: unknown }).runId
    if (suppliedRunId !== undefined) {
      if (typeof suppliedRunId !== "string" || !runIDPattern.test(suppliedRunId)) {
        throw new Error("CODEA_REVIEW_RUN_ID_INVALID")
      }
      if (suppliedRunId !== runId) throw new Error("CODEA_REVIEW_RUN_ID_MISMATCH")
    }

    if (args.action === "prepare") {
      if (!args.intent) throw new Error("CODEA_REVIEW_PREPARE_INTENT_REQUIRED")
      const argv = ["review", "prepare", "--run-id", runId, "--mode", args.intent.mode]
      if (args.intent.target?.trim()) argv.push("--target", args.intent.target.trim())
      const runtime = await invoke(worktree, argv)
      const scope = await readScope(worktree, runId)
      return JSON.stringify({ runtime, scope, hostBinding, nextAction: nextAction(runtime, scope) }, null, 2)
    }

    if (args.action === "read") {
      if (!args.source) throw new Error("CODEA_REVIEW_SOURCE_REF_REQUIRED")
      return JSON.stringify({ ...await readAuthorizedSource(worktree, runId, args.source), hostBinding }, null, 2)
    }

    if (!args.result) throw new Error("CODEA_REVIEW_FINISH_RESULT_REQUIRED")
    const requestsRoot = path.resolve(worktree, ".code-harness", "runs", runId, "requests")
    const requestPath = path.resolve(requestsRoot, `finish-${randomUUID()}.json`)
    const payload = {
      runId,
      reads: args.result.reads,
      findings: args.result.findings,
      pendingRisks: args.result.pendingRisks,
      gaps: args.result.gaps,
    }
    await exclusiveWrite(requestPath, `${JSON.stringify(payload, null, 2)}\n`)
    const relative = path.relative(worktree, requestPath).split(path.sep).join("/")
    const runtime = await invoke(worktree, ["review", "finish", "--input", relative])
    return JSON.stringify({ runtime, hostBinding }, null, 2)
  },
})
