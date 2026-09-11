import { tool } from "@opencode-ai/plugin"
import { createHash } from "node:crypto"
import { mkdir, rename, writeFile } from "node:fs/promises"
import path from "node:path"

const runID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/

function canonicalTarget(kind: "change-analysis" | "findings", id: string) {
  const file = kind === "change-analysis" ? "change-analysis-proposal.json" : "finding-proposals.json"
  const receipt = kind === "change-analysis" ? "change-analysis-reviewer-authority.json" : "finding-reviewer-authority.json"
  return { file, receipt }
}

async function atomicWrite(target: string, data: string) {
  await mkdir(path.dirname(target), { recursive: true })
  const tmp = `${target}.tmp-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}`
  await writeFile(tmp, data, { encoding: "utf8", mode: 0o644 })
  await rename(tmp, target)
}

export default tool({
  description: "Submit a Codea Harness semantic proposal from the independent Reviewer. Host identity is recorded for Runtime certification.",
  args: {
    kind: tool.schema.enum(["change-analysis", "findings"]),
    runId: tool.schema.string(),
    proposal: tool.schema.string().describe("Exact JSON payload for the semantic proposal"),
  },
  async execute(args, context) {
    if (context.agent !== "reviewer") {
      throw new Error("MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN: codea-reviewer-submit requires Host agent=reviewer")
    }
    if (!context.sessionID || !context.messageID) {
      throw new Error("REVIEWER_UNAVAILABLE: Host did not provide Reviewer session/message identity")
    }
    if (!runID.test(args.runId)) {
      throw new Error("invalid runId")
    }
    let parsed: unknown
    try {
      parsed = JSON.parse(args.proposal)
    } catch (error) {
      throw new Error(`REVIEWER_MALFORMED_OUTPUT: proposal is not JSON: ${String(error)}`)
    }
    if (args.kind === "change-analysis") {
      if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
        throw new Error("REVIEWER_MALFORMED_OUTPUT: change-analysis proposal must be a JSON object")
      }
    } else if (!Array.isArray(parsed)) {
      throw new Error("REVIEWER_MALFORMED_OUTPUT: findings proposal must be a JSON array")
    }

    const target = canonicalTarget(args.kind, args.runId)
    const requestsRoot = path.resolve(context.worktree, ".code-harness", "runs", args.runId, "requests")
    const proposalPath = path.resolve(requestsRoot, target.file)
    const receiptPath = path.resolve(requestsRoot, target.receipt)
    const proposalText = `${JSON.stringify(parsed, null, 2)}\n`
    const sha256 = createHash("sha256").update(proposalText, "utf8").digest("hex")
    const receipt = {
      version: 1,
      host: "opencode",
      source: "opencode-tool-context",
      runId: args.runId,
      proposalKind: args.kind,
      agent: context.agent,
      sessionId: context.sessionID,
      messageId: context.messageID,
      proposalPath: path.relative(context.worktree, proposalPath).split(path.sep).join("/"),
      proposalSha256: sha256,
    }

    await atomicWrite(proposalPath, proposalText)
    await atomicWrite(receiptPath, `${JSON.stringify(receipt, null, 2)}\n`)
    return `REVIEWER_PROPOSAL_SUBMITTED kind=${args.kind} runId=${args.runId} sessionId=${context.sessionID}`
  },
})
