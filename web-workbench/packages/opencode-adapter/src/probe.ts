/**
 * OpenCode 能力探测
 *
 * 职责：识别当前原生服务版本和支持的执行能力。
 * Java 研发阅读提示：能力存在与具体模型/Skill 已可执行是不同的判断。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
import type { DTO } from '../../contracts/src/domain.ts';
import type { Continuity } from '../../contracts/src/events.ts';

type CapabilityName =
  | 'service.identity' | 'service.authentication' | 'session.create' | 'session.location'
  | 'agent.trusted' | 'command.trusted' | 'skill.trusted' | 'template.trusted'
  | 'model.private' | 'command.execute' | 'permission.native' | 'messages.read'
  | 'events.read' | 'session.state' | 'report.persist' | 'discussion.same_session'
  | 'session.interrupt' | 'network.offline' | 'repository.immutable'
  | 'forms.native' | 'instruction.entry' | 'session.log';

export type Observation = {
  status: 'SUPPORTED' | 'UNSUPPORTED' | 'UNKNOWN';
  evidence: string;
  /** Correlatable ID for a real observed Windows run, NOT a fake unit-test pass. */
  windowsRunId?: string;
};

export const CAPABILITIES: ReadonlyArray<{ name: CapabilityName; required: boolean }> = [
  { name: 'service.identity', required: true },
  { name: 'service.authentication', required: true },
  { name: 'session.create', required: true },
  { name: 'session.location', required: true },
  { name: 'agent.trusted', required: true },
  { name: 'command.trusted', required: true },
  { name: 'skill.trusted', required: true },
  { name: 'template.trusted', required: true },
  { name: 'model.private', required: true },
  { name: 'command.execute', required: true },
  { name: 'permission.native', required: true },
  { name: 'messages.read', required: true },
  { name: 'events.read', required: true },
  { name: 'session.state', required: true },
  { name: 'report.persist', required: true },
  { name: 'discussion.same_session', required: true },
  { name: 'session.interrupt', required: true },
  { name: 'network.offline', required: true },
  { name: 'repository.immutable', required: true },
  { name: 'forms.native', required: true },
  { name: 'instruction.entry', required: false },
  { name: 'session.log', required: false },
] as const;

export type ProbeSummary = {
  capabilities: DTO<'Capability'>[];
  ready: boolean;
  notesEnabled: boolean;
  continuity: Continuity;
  blockers: string[];
};

export function evaluateCapabilities(observations: Partial<Record<CapabilityName, Observation>> = {}): ProbeSummary {
  const capabilities: DTO<'Capability'>[] = CAPABILITIES.map(({ name, required }) => {
    const found = observations[name];
    // A manual declaration without a linked real Windows trace is not proof.
    const verified = Boolean(found?.windowsRunId?.trim() && found?.evidence?.trim());
    return {
      name,
      required,
      status: verified ? found!.status : 'UNKNOWN',
      evidence: verified ? found!.evidence.slice(0, 1000) : 'PENDING: requires attributable Windows native evidence',
    };
  });
  const blockers = capabilities.filter(c => c.required && c.status !== 'SUPPORTED').map(c => c.name);
  const get = (name: CapabilityName) => capabilities.find(c => c.name === name)?.status;
  return {
    capabilities,
    ready: blockers.length === 0,
    blockers,
    notesEnabled: get('instruction.entry') === 'SUPPORTED',
    continuity: get('session.log') === 'SUPPORTED' ? 'COMPLETE' : 'GAP',
  };
}

export function requireReady(summary: ProbeSummary): void {
  if (!summary.ready) throw new Error('OPENCODE_CAPABILITY_BLOCKED: ' + summary.blockers.join(', '));
}
