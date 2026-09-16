import { tool } from "@opencode-ai/plugin"
import { execFile } from "node:child_process"
import { mkdir, readFile, rename, writeFile } from "node:fs/promises"
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

async function atomicWrite(target: string, data: string) {
  await mkdir(path.dirname(target), { recursive: true })
  const tmp = `${target}.tmp-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}`
  await writeFile(tmp, data, { encoding: "utf8", mode: 0o644 })
  await rename(tmp, target)
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

export default tool({
  description: "Codea Harness 1.8 primary review tool. Prepare bounded chains, verify a real user selection from Host context, or finish the durable report.",
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
      return JSON.stringify({ runtime, scope }, null, 2)
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
      return JSON.stringify({ runtime, scope }, null, 2)
    }

    if (!args.result) throw new Error("CODEA_REVIEW_FINISH_RESULT_REQUIRED")
    const requestsRoot = path.resolve(worktree, ".code-harness", "runs", args.runId, "requests")
    const requestPath = path.resolve(requestsRoot, `finish-${context.messageID || Date.now()}.json`)
    const payload = {
      runId: args.runId,
      reads: args.result.reads,
      findings: args.result.findings,
      pendingRisks: args.result.pendingRisks,
      gaps: args.result.gaps,
    }
    await atomicWrite(requestPath, `${JSON.stringify(payload, null, 2)}\n`)
    const relative = path.relative(worktree, requestPath).split(path.sep).join("/")
    const runtime = await invoke(worktree, ["review", "finish", "--input", relative])
    return JSON.stringify({ runtime }, null, 2)
  },
})
