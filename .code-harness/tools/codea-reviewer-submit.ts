import { tool } from "@opencode-ai/plugin"
import { createHash } from "node:crypto"
import { mkdir, rename, writeFile } from "node:fs/promises"
import path from "node:path"

const runID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/

type ObjectValue = Record<string, unknown>
function object(value: unknown, label: string): ObjectValue {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(`PROPOSAL_PREFLIGHT_FAILED: ${label} must be an object`)
  return value as ObjectValue
}
function text(value: unknown, label: string): string {
  if (typeof value !== "string" || !value.trim()) throw new Error(`PROPOSAL_PREFLIGHT_FAILED: ${label} must be nonempty text`)
  return value.trim()
}
function array(value: unknown, label: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`PROPOSAL_PREFLIGHT_FAILED: ${label} must be an array`)
  return value
}
function ref(value: unknown, label: string) {
  const r = object(value, label)
  const source = text(r.path, `${label}.path`).replaceAll("\\", "/")
  const file = path.posix.normalize(source)
  if (source.startsWith("/") || file === "." || file === ".." || file.startsWith("../")) throw new Error(`PROPOSAL_PREFLIGHT_FAILED: invalid ${label}.path`)
  const symbol = text(r.symbol, `${label}.symbol`)
  const workspace = r.workspace === undefined ? "current" : text(r.workspace, `${label}.workspace`)
  return { symbol, key: `${workspace}\0${file}\0${symbol}` }
}

// This is a cheap submission check, not certification. Runtime subsequently
// validates the complete schema, live evidence, coverage and Host attestation.
function preflight(kind: string, value: unknown) {
  if (kind === "selection") return
  if (kind === "findings") {
    for (const [i, item] of array(value, "findings").entries()) {
      const finding = object(item, `findings[${i}]`)
      for (const key of ["proposalId", "reviewUnitId", "ruleId", "category", "severity", "problem", "impact", "recommendation"]) text(finding[key], `findings[${i}].${key}`)
      object(finding.anchor, `findings[${i}].anchor`)
      array(finding.evidenceRefs, `findings[${i}].evidenceRefs`)
      if (typeof finding.needsTest !== "boolean" || typeof finding.introducedByChange !== "boolean" || typeof finding.confidence !== "number") throw new Error(`PROPOSAL_PREFLIGHT_FAILED: findings[${i}] has invalid flags/confidence`)
    }
    return
  }
  const proposal = object(value, "change-analysis")
  for (const key of ["changedFileRoles", "affectedControllers", "callChains", "symbolLocations", "resourceRelations", "externalDependencies", "riskAreas"]) array(proposal[key], key)
  const coverage = object(proposal.reviewCoverage, "reviewCoverage")
  text(coverage.status, "reviewCoverage.status")
  array(coverage.reviewedFiles, "reviewCoverage.reviewedFiles")
  array(coverage.unresolvedSymbols, "reviewCoverage.unresolvedSymbols")
  const exact = new Set<string>()
  const symbols = new Map<string, Set<string>>()
  for (const location of proposal.symbolLocations as unknown[]) {
    const r = ref(location, "symbolLocation")
    exact.add(r.key)
    const keys = symbols.get(r.symbol) ?? new Set<string>()
    keys.add(r.key)
    symbols.set(r.symbol, keys)
  }
  function resolve(symbolValue: unknown, reference: unknown, label: string) {
    const symbol = text(symbolValue, label)
    if (reference !== undefined) {
      const r = ref(reference, label)
      if (r.symbol !== symbol || !exact.has(r.key)) throw new Error(`PROPOSAL_PREFLIGHT_FAILED: ${label} must match its symbol and exact symbolLocation`)
    } else if (symbols.get(symbol)?.size !== 1) {
      throw new Error(`PROPOSAL_PREFLIGHT_FAILED: ${label} has missing or ambiguous symbolLocations; provide exact refs`)
    }
  }
  for (const [i, value] of (proposal.callChains as unknown[]).entries()) {
    const chain = object(value, `callChains[${i}]`)
    const nodes = array(chain.chain, `callChains[${i}].chain`)
    const refs = chain.chainRefs === undefined ? [] : array(chain.chainRefs, `callChains[${i}].chainRefs`)
    if (refs.length && refs.length !== nodes.length) throw new Error(`PROPOSAL_PREFLIGHT_FAILED: callChains[${i}] chainRefs length=${refs.length} chain length=${nodes.length}; provide one ref per node in order, then resubmit this run`)
    resolve(chain.entryPoint, chain.entryPointRef, `callChains[${i}].entryPointRef`)
    nodes.forEach((node, j) => resolve(node, refs[j], `callChains[${i}].chainRefs[${j}]`))
  }
}

function canonicalTarget(kind: "change-analysis" | "findings" | "selection", id: string) {
  if (kind === "selection") return { file: "review-selection.json", receipt: "review-selection-authority.json" }
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
  description: "Submit main Agent review analysis, findings, or a real user selection. The historical tool name is retained for upgrade compatibility; no Reviewer subagent is required.",
  args: {
    kind: tool.schema.enum(["change-analysis", "findings", "selection"]),
    runId: tool.schema.string(),
    proposal: tool.schema.string().describe("Exact JSON payload for the semantic proposal"),
  },
  async execute(args, context) {
    if (!context.agent) throw new Error("PRIMARY_AGENT_IDENTITY_MISSING")
    if (args.kind === "selection" && context.agent === "reviewer") throw new Error("HUMAN_SELECTION_REQUIRES_PRIMARY_AGENT")
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
    if (args.kind !== "findings") {
      if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
        throw new Error("REVIEWER_MALFORMED_OUTPUT: change-analysis proposal must be a JSON object")
      }
    } else if (!Array.isArray(parsed)) {
      throw new Error("REVIEWER_MALFORMED_OUTPUT: findings proposal must be a JSON array")
    }

    preflight(args.kind, parsed)
    const target = canonicalTarget(args.kind, args.runId)
    const requestsRoot = path.resolve(context.worktree, ".code-harness", "runs", args.runId, "requests")
    const proposalPath = path.resolve(requestsRoot, target.file)
    const receiptPath = path.resolve(requestsRoot, target.receipt)
    const proposalText = `${JSON.stringify(parsed, null, 2)}\n`
    const sha256 = createHash("sha256").update(proposalText, "utf8").digest("hex")
    const receipt = {
      version: context.agent === "reviewer" ? 1 : 2,
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
