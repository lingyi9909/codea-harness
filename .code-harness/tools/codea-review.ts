import { tool } from "@opencode-ai/plugin"
import { execFile } from "node:child_process"
import { randomUUID } from "node:crypto"
import { mkdir, readFile, writeFile } from "node:fs/promises"
import path from "node:path"
import { promisify } from "node:util"

const execFileAsync = promisify(execFile)
const runIDPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/

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

function selectionMenuText(runtime: Record<string, unknown>) {
  const runId = String(runtime.runId ?? "").trim()
  const optionsHash = String(runtime.optionsHash ?? "").trim()
  const chains = Array.isArray(runtime.chains) ? runtime.chains : []
  const lines = chains.map((item) => {
    const chain = item as Record<string, unknown>
    return `${String(chain.id ?? "").trim()} ${String(chain.name ?? "").trim()}`.trim()
  }).filter(Boolean)
  if (!runId || !optionsHash || lines.length === 0) return ""
  return [`${runId} options=${optionsHash}`, ...lines].join("\n")
}

function nextAction(runtime: Record<string, unknown>, scope?: Record<string, unknown>) {
  if (runtime.selectionRequired === true) {
    const requiredMenuText = selectionMenuText(runtime)
    return {
      type: "WAIT_FOR_REAL_USER_SELECTION",
      mandatory: true,
      assistantTurnTerminal: true,
      requiredMenuText,
      instruction: "Render requiredMenuText verbatim as one plain-text/code block, with the exact '<runId> options=<optionsHash>' header and exact 'C<n> <name>' lines. Do not convert it to a Markdown table, relabel the hash, reorder chains, or omit the header. Then end this assistant turn and wait for the next real user message. Do not call select or finish yet.",
    }
  }
  if (scope) {
    return {
      type: "READ_SCOPE_AND_FINISH_THIS_TURN",
      mandatory: true,
      assistantTurnTerminal: false,
      mustContinueToolExecution: true,
      finishRequiredEvenWhenFindingsEmpty: true,
      instruction: "Read only the authorized scope.reads needed for evidence, review the selected scope, then call codea-review finish in this same assistant turn. Do not end the turn after prepare/select. findings=[] still requires finish. pendingRisks must describe only a current unresolved harmful condition supported by current source whose confirmation needs evidence outside the authorized scope; do not report hypothetical future edits, future endpoint repurposing, or generic best-practice concerns.",
    }
  }
  return {
    type: "KEEP_REPORT_INCOMPLETE",
    mandatory: true,
    instruction: "No review scope is ready. Report the concrete prepare/select state and keep the durable report INCOMPLETE; do not manufacture findings or call finish without an authorized scope.",
  }
}

export default tool({
  description: "Codea Harness 1.8 primary review tool. Prepare bounded chains, verify a real user selection from Host context, or finish the durable report. Always obey the returned nextAction. For WAIT_FOR_REAL_USER_SELECTION, copy requiredMenuText verbatim; for a ready scope, do not end the assistant turn before finish, even when findings are empty. pendingRisks are only current unresolved harmful conditions supported by current source and requiring out-of-scope confirmation, never hypothetical future changes.",
  args: {
    action: tool.schema.enum(["prepare", "select", "finish"]),
    runId: tool.schema.string(),
    intent: tool.schema.object({
      mode: tool.schema.enum(["CHANGES", "CURRENT_IMPLEMENTATION"]),
      target: tool.schema.string().optional(),
    }).optional(),
    selection: tool.schema.object({
      optionsHash: tool.schema.string(),
      ids: tool.schema.array(tool.schema.string()).min(1),
    }).optional(),
    result: tool.schema.object({
      reads: tool.schema.array(readRef),
      findings: tool.schema.array(finding),
      pendingRisks: tool.schema.array(tool.schema.string()),
      gaps: tool.schema.array(tool.schema.string()),
    }).optional(),
  },
  async execute(args, context) {
    if (!runIDPattern.test(args.runId)) throw new Error("CODEA_REVIEW_RUN_ID_INVALID")
    const worktree = path.resolve(context.worktree)

    if (args.action === "prepare") {
      if (!args.intent) throw new Error("CODEA_REVIEW_PREPARE_INTENT_REQUIRED")
      const argv = ["review", "prepare", "--run-id", args.runId, "--mode", args.intent.mode]
      if (args.intent.target?.trim()) argv.push("--target", args.intent.target.trim())
      const runtime = await invoke(worktree, argv)
      const scope = await readScope(worktree, args.runId)
      return JSON.stringify({ runtime, scope, nextAction: nextAction(runtime, scope) }, null, 2)
    }

    if (args.action === "select") {
      if (!args.selection) throw new Error("CODEA_REVIEW_SELECTION_REQUIRED")
      if (!context.sessionID || !context.messageID) throw new Error("HUMAN_SELECTION_REQUIRED: Host context unavailable")
      const runtime = await invoke(worktree, [
        "review", "select",
        "--run-id", args.runId,
        "--options-hash", args.selection.optionsHash,
        "--ids", args.selection.ids.join(","),
        "--session-id", context.sessionID,
        "--message-id", context.messageID,
      ])
      const scope = await readScope(worktree, args.runId)
      return JSON.stringify({ runtime, scope, nextAction: nextAction(runtime, scope) }, null, 2)
    }

    if (!args.result) throw new Error("CODEA_REVIEW_FINISH_RESULT_REQUIRED")
    const requestsRoot = path.resolve(worktree, ".code-harness", "runs", args.runId, "requests")
    const requestPath = path.resolve(requestsRoot, `finish-${randomUUID()}.json`)
    const payload = {
      runId: args.runId,
      reads: args.result.reads,
      findings: args.result.findings,
      pendingRisks: args.result.pendingRisks,
      gaps: args.result.gaps,
    }
    await exclusiveWrite(requestPath, `${JSON.stringify(payload, null, 2)}\n`)
    const relative = path.relative(worktree, requestPath).split(path.sep).join("/")
    const runtime = await invoke(worktree, ["review", "finish", "--input", relative])
    return JSON.stringify({ runtime }, null, 2)
  },
})
