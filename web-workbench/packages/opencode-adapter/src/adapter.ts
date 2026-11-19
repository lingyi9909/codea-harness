import { isDeepStrictEqual } from 'node:util';
import { randomUUID } from 'node:crypto';
import { realpath } from 'node:fs/promises';
import { isAbsolute } from 'node:path';
import type { DTO, NativeAcceptance, NativeInspection, SessionBinding, Snapshot, UUID } from '../../contracts/src/domain.ts';
import type { NativeEnvelope, NativeObservation } from '../../contracts/src/events.ts';
import type { OpenCodeAdapter,AnswerExpectation } from '../../contracts/src/ports.ts';
import { REVIEW_COMMAND, AGENT_ID, reviewArguments, assertNativeId } from './mapping.ts';
import { LocalNativeTransport } from './native.ts';
import { evaluateCapabilities, requireReady, type Observation } from './probe.ts';

type NativeRecord = Record<string, unknown>;
type InterKind = 'PERMISSION' | 'FORM';
type BoundState = { binding: SessionBinding; runId?: UUID; answered: Set<string>;
  requests: Map<string, { nativeId: string; kind: InterKind }> };
export type AdapterOptions = {
  transport: LocalNativeTransport;
  /** Decode only public assistant text, safe tool metadata and attributable execution evidence. */
  decodeEvent?(type: string, properties: NativeRecord): NativeObservation | undefined;
  /** Read-only native proof from the pinned build, including accepted input ownership. */
  inspectBinding?(binding:SessionBinding):Promise<{sessionId:string;serviceIdentity:string;locationKey:string;
    inputId:string;externalMutation:boolean}>;
  decodeMessage?(message:NativeRecord):Extract<NativeObservation,{kind:'message'}>|undefined;
  decodeTerminal?(state:NativeRecord):Extract<NativeObservation,{kind:'terminal'}>|undefined;
  /** T04 must independently establish clean, pinned and immutable snapshot before any mutation. */
  verifySnapshot(snapshot: Snapshot): Promise<void>;
  /** Only externally captured T01 company evidence may enable production execution. */
  observations?: Parameters<typeof evaluateCapabilities>[0];
  /** Native permission/form payload schemas are tied to the pinned company build. */
  mapInteraction(raw: unknown, kind: InterKind, context: {
    taskId: UUID; runId: UUID; businessId: UUID;
  }): DTO<'Interaction'>;
  /** The map returns native request ID, not a Harness UUID. */
  getNativeRequestId(raw: unknown): string;
  /** Optional verified T06 audit: absent/failed audit keeps externalMutation=true. */
  auditExternalWriter?(binding: SessionBinding): Promise<boolean>;
};

function obj(value: unknown): NativeRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('NATIVE_RESPONSE_SCHEMA_MISMATCH');
  return value as NativeRecord;
}
function payload(body: unknown): unknown {
  const root = obj(body);
  return 'data' in root ? root.data : root;
}
function ensure2xx(code: number, name: string): void {
  if (code < 200 || code >= 300) throw new Error(name + '_HTTP_' + code);
}
function iso(input: unknown): string {
  if (typeof input !== 'string' || Number.isNaN(Date.parse(input)))
    throw new Error('NATIVE_MESSAGE_DATE_UNMAPPED');
  return input;
}
function requiredString(value: unknown, name: string): string {
  if (typeof value !== 'string' || !value.trim()) throw new Error(name + '_UNMAPPED');
  return value;
}
function list(body: unknown): unknown[] {
  const data = payload(body);
  if (Array.isArray(data)) return data;
  const value = obj(data).items;
  if (Array.isArray(value)) return value;
  throw new Error('NATIVE_COLLECTION_SCHEMA_MISMATCH');
}
function commandAcceptance(code: number, body: unknown): NativeAcceptance {
  if (code === 400 || code === 401 || code === 403 || code === 404 || code === 409)
    return { state: 'REJECTED', reason: 'NATIVE_REQUEST_REJECTED_HTTP_' + code };
  // A 204 simply means accepted at HTTP boundary; actual model dispatch may be unknown.
  if (code >= 200 && code < 300) {
    const data = body && typeof body === 'object' ? obj(body).data : null;
    const id = data && typeof data === 'object' ? obj(data).id : undefined;
    if (typeof id === 'string' && id) return { state: 'ACCEPTED', inputId: id };
    return { state: 'UNKNOWN', reason: 'NATIVE_ACCEPTANCE_UNCONFIRMED_HTTP_' + code };
  }
  return { state: 'UNKNOWN', reason: 'NATIVE_UNCERTAIN_HTTP_' + code };
}

/** Bound, non-retrying OpenCode 2.x adapter. T06 supplies durable bindings and outbox. */
export class HttpOpenCodeAdapter implements OpenCodeAdapter {
  private readonly sessions = new Map<string, BoundState>();
  private readonly ids = new Map<string, UUID>();
  private readonly options: AdapterOptions;
  constructor(options: AdapterOptions) { this.options = options; }

  async probe(): Promise<DTO<'Capability'>[]> {
    const caps=evaluateCapabilities(this.options.observations).capabilities;
    const liveOnly=!caps.some(c=>c.name==='session.log'&&c.status==='SUPPORTED');
    if(!this.options.decodeEvent || (liveOnly && (!this.options.decodeMessage || !this.options.decodeTerminal)))
      return caps.map(c=>c.name==='events.read'?{...c,status:'UNKNOWN',evidence:'Pinned-build observation decoder required'}:c);
    return caps;
  }
  private ready(): void { requireReady(evaluateCapabilities(this.options.observations)); }
  private state(binding: SessionBinding): BoundState {
    const saved = this.sessions.get(binding.sessionId);
    if (!saved || saved.binding.taskId !== binding.taskId ||
        saved.binding.locationKey !== binding.locationKey ||
        saved.binding.serviceIdentity !== binding.serviceIdentity)
      throw new Error('SESSION_BINDING_NOT_TRUSTED');
    return saved;
  }
  async createSession(taskId: UUID, snapshot: Snapshot): Promise<SessionBinding> {
    this.ready();
    if (taskId !== snapshot.taskId || !isAbsolute(snapshot.storageKey))
      throw new Error('SNAPSHOT_BINDING_INVALID');
    await this.options.verifySnapshot(snapshot);
    const location = await realpath(snapshot.storageKey);
    const response = await this.options.transport.call('createSession', 'POST', {
      location: { directory: location }, agent: AGENT_ID,
      title: 'Code Harness Review',
    });
    ensure2xx(response.status, 'CREATE_SESSION');
    const native = obj(payload(response.body));
    const id = assertNativeId(native.id);
    const loc = obj(native.location);
    if (loc.directory !== location) throw new Error('NATIVE_SESSION_LOCATION_MISMATCH');
    if (typeof native.agent === 'string' && native.agent !== AGENT_ID)
      throw new Error('NATIVE_SESSION_AGENT_MISMATCH');
    const binding: SessionBinding = { taskId, serviceIdentity: this.options.transport.serviceIdentity,
      sessionId: id, locationKey: location };
    this.sessions.set(id, { binding, answered: new Set(), requests: new Map() });
    return binding;
  }
  async reattach(binding:SessionBinding,runId:string,inputId:string):Promise<boolean>{
    this.ready();
    if(!this.options.inspectBinding || binding.serviceIdentity!==this.options.transport.serviceIdentity ||
      !this.options.auditExternalWriter)return false;
    const proof=await this.options.inspectBinding(binding);
    if(proof.sessionId!==binding.sessionId || proof.locationKey!==binding.locationKey ||
      proof.serviceIdentity!==binding.serviceIdentity || proof.inputId!==inputId || proof.externalMutation ||
      await this.options.auditExternalWriter(binding)!==false)return false;
    const existing=this.sessions.get(binding.sessionId);
    if(existing){
      this.state(binding);
      return existing.runId===runId;
    }
    this.sessions.set(binding.sessionId,{binding,runId,answered:new Set(),requests:new Map()});
    return true;
  }
  async putContext(binding: SessionBinding, context: { scope: DTO<'Scope'>; notes?: string }): Promise<void> {
    this.ready();
    this.state(binding);
    if (context.notes && context.notes.length > 2000) throw new Error('NOTES_TOO_LONG');
    if (!context.notes) return;
    // No alternate prompt: missing instruction entry must result in disabled notes.
    if (evaluateCapabilities(this.options.observations).notesEnabled === false)
      throw new Error('INSTRUCTION_ENTRY_NOT_SUPPORTED');
    const input = JSON.stringify({ schema: 'harness-scope-v1', scope: context.scope, notes: context.notes });
    const response = await this.options.transport.call('instructions', 'PUT',
      { text: input }, { sessionID: binding.sessionId, key: 'code-harness-scope' });
    ensure2xx(response.status, 'PUT_CONTEXT');
  }
  async startReview(binding: SessionBinding, input: { runId: UUID; controller?: string; recheck?: boolean }): Promise<NativeAcceptance> {
    this.ready();
    const state = this.state(binding);
    if (input.recheck) {
      if (!state.runId || state.runId === input.runId)
        throw new Error('RECHECK_REQUIRES_PREVIOUS_RUN');
      const inspection = await this.inspect(binding);
      if (inspection.active !== false || inspection.pendingInputs !== 0 ||
          inspection.pendingInteractions !== 0 || inspection.externalMutation)
        throw new Error('RECHECK_SESSION_NOT_PROVEN_IDLE');
    } else if (state.runId) throw new Error('SESSION_RUN_ALREADY_DISPATCHED');
    const args = reviewArguments(input.controller);
    // Record before dispatch, because transport timeouts are UNKNOWN and must not be retried.
    state.runId = input.runId;
    try {
      const result = await this.options.transport.call('command', 'POST', {
        name: REVIEW_COMMAND, text: args,
      }, { sessionID: binding.sessionId });
      return commandAcceptance(result.status, result.body);
    } catch {
      return { state: 'UNKNOWN', reason: 'NATIVE_COMMAND_TRANSPORT_UNCERTAIN_RECONCILE' };
    }
  }
  async discuss(binding: SessionBinding, input: { runId: UUID; messageId: UUID; text: string; quote?: string; reportArtifactId?: string }): Promise<NativeAcceptance> {
    this.ready();
    const state = this.state(binding);
    if (!state.runId) throw new Error('REVIEW_NOT_STARTED');
    if (!input.text.trim() || input.text.length > 8000) throw new Error('DISCUSSION_TEXT_INVALID');
    if (state.runId === input.runId) throw new Error('DISCUSSION_RUN_MUST_BE_NEW');
    if (input.quote && (input.quote.length > 4000 || !input.quote.trim()))
      throw new Error('DISCUSSION_QUOTE_INVALID');
    if (input.reportArtifactId && input.reportArtifactId.length > 128)
      throw new Error('DISCUSSION_ARTIFACT_ID_INVALID');
    const promptText = input.quote
      ? '引用的历史报告原文（仅作用户参考，不更改代码范围）：\\n'+
        input.quote+'\\n\\n当前追问：\\n'+input.text
      : input.text;
    state.runId = input.runId;
    try {
      const response = await this.options.transport.call('prompt', 'POST', {
        text: promptText, delivery: 'queue',
        metadata: { harnessRunId: input.runId, harnessMessageId: input.messageId },
      }, { sessionID: binding.sessionId });
      return commandAcceptance(response.status, response.body);
    } catch {
      return { state: 'UNKNOWN', reason: 'NATIVE_PROMPT_TRANSPORT_UNCERTAIN_RECONCILE' };
    }
  }
  async inspect(binding: SessionBinding): Promise<NativeInspection> {
    const state = this.state(binding);
    let active: NativeInspection['active'] = 'UNKNOWN';
    let pendingInputs: NativeInspection['pendingInputs'] = 'UNKNOWN';
    let pendingInteractions: NativeInspection['pendingInteractions'] = 'UNKNOWN';
    const acceptedInputIds: string[] = [];
    try {
      const response = await this.options.transport.call('active', 'GET');
      ensure2xx(response.status, 'NATIVE_ACTIVE');
      const data = payload(response.body);
      if (data && typeof data === 'object' && !Array.isArray(data))
        active = Object.hasOwn(data, binding.sessionId);
    } catch { /* Stay UNKNOWN; no idle inference. */ }
    try {
      const response = await this.options.transport.call('inbox', 'GET', undefined,
        { sessionID: binding.sessionId });
      ensure2xx(response.status, 'NATIVE_INBOX');
      const items = list(response.body);
      pendingInputs = items.length;
      for (const item of items) {
        const id = obj(item).id;
        if (typeof id === 'string') acceptedInputIds.push(id);
      }
    } catch { /* Stay UNKNOWN; do not replay a command. */ }
    try {
      const interactions = await this.listInteractions(binding);
      pendingInteractions = interactions.length;
    } catch { /* Native schema/build not verified. */ }
    let externalMutation = true;
    if (this.options.auditExternalWriter) {
      try { externalMutation = await this.options.auditExternalWriter(binding); }
      catch { externalMutation = true; }
    }
    return { active, pendingInputs, pendingInteractions, acceptedInputIds, externalMutation };
  }
  async readMessages(binding: SessionBinding, cursor?: string): Promise<{
    items: DTO<'Message'>[]; nextCursor: string | null;
  }> {
    const state = this.state(binding);
    if (!state.runId) throw new Error('RUN_BINDING_MISSING');
    const response = await this.options.transport.call('messages', 'GET', undefined,
      { sessionID: binding.sessionId }, cursor ? { cursor } : {});
    ensure2xx(response.status, 'READ_MESSAGES');
    const collection = payload(response.body);
    const values = Array.isArray(collection) ? collection : list(response.body);
    const next = !Array.isArray(collection) ? obj(collection).cursor : undefined;
    return {
      items: values.map(value => {
        const message = obj(value);
        const nativeId = requiredString(message.id, 'NATIVE_MESSAGE_ID');
        const role = message.role === 'user' ? 'USER' :
          message.role === 'assistant' ? 'ASSISTANT' : null;
        if (!role) throw new Error('NATIVE_MESSAGE_ROLE_UNMAPPED');
        let text: string;
        if (typeof message.text === 'string') text = message.text;
        else if (Array.isArray(message.parts))
          text = message.parts.filter((part): part is NativeRecord =>
            part !== null && typeof part === 'object' && !Array.isArray(part))
            .filter(part => part.type === 'text' && typeof part.text === 'string')
            .map(part => part.text as string).join('\n');
        else throw new Error('NATIVE_MESSAGE_PARTS_UNMAPPED');
        const createdAt = iso(message.createdAt);
        const updatedAt = iso(message.updatedAt);
        const status = message.status;
        if (status !== 'DRAFT' && status !== 'STREAMING' &&
            status !== 'COMPLETE' && status !== 'INTERRUPTED')
          throw new Error('NATIVE_MESSAGE_STATUS_UNMAPPED');
        const key = binding.sessionId + ':' + nativeId;
        let id = this.ids.get(key);
        if (!id) { id = randomUUID(); this.ids.set(key, id); }
        return { id, runId: state.runId!, role, text, status,
          reportArtifactId: null, createdAt, updatedAt };
      }),
      nextCursor: typeof next === 'string' ? next : null,
    };
  }
  async *recoverObservations(binding:SessionBinding,inputId:string,signal:AbortSignal):AsyncIterable<NativeObservation>{
    const state=this.state(binding),runId=state.runId;
    const {decodeMessage,decodeTerminal}=this.options;
    if(!runId || !decodeMessage || !decodeTerminal)throw new Error('NATIVE_HISTORY_DECODER_REQUIRED');
    const route={sessionID:binding.sessionId};
    const inspection=await this.options.transport.call('state','GET',undefined,route);
    ensure2xx(inspection.status,'NATIVE_STATE');
    const terminal=decodeTerminal(obj(payload(inspection.body)));
    const seen=new Set<string>();
    let cursor:string|undefined;
    do{
      signal.throwIfAborted();
      const response=await this.options.transport.call('messages','GET',undefined,route,cursor?{cursor}:{});
      ensure2xx(response.status,'NATIVE_MESSAGES');
      if(state.runId!==runId)throw new Error('NATIVE_EVENT_RUN_CHANGED');
      for(const raw of list(response.body)){
        const observation=decodeMessage(obj(raw));
        if(observation?.inputId===inputId)yield observation;
      }
      const page=payload(response.body);
      const next=Array.isArray(page)?null:obj(page).cursor;
      if(next==null)break;
      if(typeof next!=='string'||!next||seen.has(next))throw new Error('NATIVE_MESSAGE_CURSOR_INVALID');
      seen.add(next);cursor=next;
    }while(true);
    if(terminal?.inputId===inputId)yield terminal;
  }
  async *subscribe(binding: SessionBinding, cursor?: string, signal?: AbortSignal): AsyncIterable<NativeEnvelope> {
    const state = this.state(binding);
    if (!state.runId) throw new Error('RUN_BINDING_MISSING');
    const subscribedRunId = state.runId;
    // No pretending live SSE can replay after disconnect; cursor requires durable native log.
    const route = cursor ? 'log' : 'events';
    const replacements: Record<string, string> = cursor ? { sessionID: binding.sessionId } : {};
    const query: Record<string, string> = cursor ? { after: cursor, follow: 'true' } : {};
    for await (const frame of this.options.transport.stream(route, replacements, query, signal)) {
      if (state.runId !== subscribedRunId) throw new Error('NATIVE_EVENT_RUN_CHANGED');
      const event = obj(frame.data);
      const props = event.properties && typeof event.properties === 'object' ? obj(event.properties) : event;
      const eventSession = props.sessionID ?? event.sessionID;
      if (eventSession !== binding.sessionId) continue;
      const type = requiredString(event.type ?? frame.event, 'NATIVE_EVENT_TYPE');
      yield { observation: this.options.decodeEvent?.(type, props), sourceId: frame.id, cursor: frame.id, taskId: binding.taskId,
        runId: subscribedRunId, type,
        data: props };
    }
  }
  async interrupt(binding: SessionBinding): Promise<{ responseReceived: boolean; interrupted?: boolean }> {
    this.state(binding);
    try {
      const response = await this.options.transport.call('interrupt', 'POST', undefined,
        { sessionID: binding.sessionId });
      if (response.status !== 200) return { responseReceived: false };
      const data = payload(response.body);
      const interrupted = obj(data).interrupted;
      return { responseReceived: true,
        interrupted: typeof interrupted === 'boolean' ? interrupted : undefined };
    } catch { return { responseReceived: false }; }
    // State reconciliation is separate: interrupted=true does not prove stopped.
  }
  async listInteractions(binding: SessionBinding): Promise<Array<DTO<'Interaction'> & { nativeRequestId: string }>> {
    const state = this.state(binding);
    if (!state.runId) throw new Error('RUN_BINDING_MISSING');
    const results: Array<DTO<'Interaction'> & {nativeRequestId: string}> = [];
    for (const [route, kind] of [['permissions', 'PERMISSION'], ['forms', 'FORM']] as const) {
      const response = await this.options.transport.call(route, 'GET', undefined,
        { sessionID: binding.sessionId });
      ensure2xx(response.status, 'NATIVE_' + kind);
      for (const raw of list(response.body)) {
        const entry = obj(raw);
        if (entry.sessionID !== binding.sessionId)
          throw new Error('NATIVE_INTERACTION_SESSION_MISMATCH');
        const nativeId = assertNativeId(this.options.getNativeRequestId(raw));
        const existing = [...state.requests.entries()].find(([, req]) => req.nativeId === nativeId && req.kind === kind);
        const businessId = existing?.[0] ?? randomUUID();
        const interaction = this.options.mapInteraction(raw, kind,
          { taskId: binding.taskId, runId: state.runId, businessId });
        if (interaction.taskId !== binding.taskId || interaction.runId !== state.runId ||
            interaction.kind !== kind || interaction.id !== businessId ||
            interaction.status !== 'PENDING') throw new Error('NATIVE_INTERACTION_SCOPE_INVALID');
        state.requests.set(businessId, { nativeId, kind });
        results.push({...interaction,nativeRequestId:nativeId});
      }
    }
    return results;
  }
  async answer(binding: SessionBinding, interactionId: string,
    answer: DTO<'AnswerRequest'>['answer'],expected?:AnswerExpectation): Promise<NativeAcceptance> {
    this.ready();
    const state = this.state(binding);
    const req = state.requests.get(interactionId);
    if (!req || state.answered.has(interactionId) || req.kind !== answer.kind)
      return { state: 'REJECTED', reason: 'INTERACTION_NOT_PENDING_OR_NOT_OWNED' };
    // Re-read pending native requests; reject if request already settled or changed.
    const pending = await this.listInteractions(binding);
    const current=pending.find(i=>i.id===interactionId&&i.kind===answer.kind);
    if (!current)
      return { state: 'REJECTED', reason: 'NATIVE_INTERACTION_EXPIRED' };
    if(expected){
      const schema=current.kind==='FORM'?{title:current.title,fields:current.fields}:
        {tool:current.tool,requestText:current.requestText,allowedDecisions:current.allowedDecisions};
      if(current.nativeRequestId!==expected.nativeRequestId || current.revision!==expected.revision ||
        Date.parse(current.expiresAt)<=Date.now() || !isDeepStrictEqual(schema,expected.schema))
        return {state:'REJECTED',reason:'NATIVE_INTERACTION_CHANGED'};
    }
    state.answered.add(interactionId); // Prevent double-send even when transport fails.
    const route = answer.kind === 'PERMISSION' ? 'permissionReply' : 'formReply';
    const body = answer.kind === 'PERMISSION'
      ? { decision: answer.decision === 'ALLOW_ONCE' ? 'once' : 'reject' }
      : { cancel: answer.cancel, values: answer.values };
    try {
      const response = await this.options.transport.call(route, 'POST', body,
        { sessionID: binding.sessionId, requestID: req.nativeId, formID: req.nativeId });
      return commandAcceptance(response.status, response.body);
    } catch {
      return { state: 'UNKNOWN', reason: 'NATIVE_INTERACTION_RESPONSE_UNCERTAIN_RECONCILE' };
    }
  }
}
