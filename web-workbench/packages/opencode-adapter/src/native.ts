import { createHash } from 'node:crypto';
import { lstat, readFile, realpath, stat } from 'node:fs/promises';
import { dirname, relative, resolve, isAbsolute } from 'node:path';
import { REPORT_RELATIVE_PATH } from './mapping.ts';

export type NativeMethod = 'GET' | 'POST' | 'PUT';
export type NativeRoute = 'identity' | 'createSession' | 'session' | 'command'
  | 'messages' | 'events' | 'state' | 'permissions' | 'permissionReply'
  | 'forms' | 'formReply' | 'prompt' | 'interrupt' | 'instructions' | 'log'
  | 'active' | 'inbox' | 'resources';
export type NativeWireManifest = {
  /** Filled from actual company OpenCode build, not presumed public SDK methods. */
  build: string;
  serviceIdentity: string;
  baseUrl: string;
  paths: Partial<Record<NativeRoute, string>>;
  /** Auth token comes ONLY from environment; never serialize it into this file. */
  authHeader?: string;
};

/** Loopback-only, redirect-free HTTP transport. No implicit retries, including GET. */
export class LocalNativeTransport {
  private readonly manifest: NativeWireManifest;
  readonly serviceIdentity: string;
  private readonly token?: string;
  constructor(manifest: NativeWireManifest, token?: string) {
    this.manifest = manifest;
    this.serviceIdentity = manifest.serviceIdentity;
    this.token = token;
    const url = new URL(manifest.baseUrl);
    if (url.protocol !== 'http:' ||
        !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname) ||
        url.username || url.password || url.search || url.hash)
      throw new Error('NATIVE_ENDPOINT_NOT_LOOPBACK');
    if (!manifest.build.trim() || !manifest.serviceIdentity.trim()) throw new Error('NATIVE_IDENTITY_NOT_PINNED');
  }

  async call(route: NativeRoute, method: NativeMethod, payload?: unknown,
    replacements: Record<string, string> = {}, query: Record<string, string> = {}): Promise<{ status: number; body: unknown }> {
    let path = this.manifest.paths[route];
    if (!path || !path.startsWith('/api/')) throw new Error('NATIVE_ROUTE_NOT_VERIFIED: ' + route);
    for (const [name, raw] of Object.entries(replacements)) {
      if (!/^[a-zA-Z][a-zA-Z0-9]*$/.test(name) || !raw || raw.length > 512)
        throw new Error('INVALID_NATIVE_ROUTE_PARAMETER');
      path = path.replace('{' + name + '}', encodeURIComponent(raw));
    }
    if (path.includes('{') || path.includes('}') || path.includes('..') || path.includes('?') || path.includes('#'))
      throw new Error('UNSAFE_NATIVE_ROUTE');
    const base = new URL(this.manifest.baseUrl);
    const url = new URL(path, base);
    if (url.origin !== base.origin) throw new Error('NATIVE_ORIGIN_MISMATCH');
    for (const [key, value] of Object.entries(query)) {
      if (!/^[a-zA-Z][a-zA-Z0-9.]*$/.test(key) || value.length > 4096)
        throw new Error('INVALID_NATIVE_QUERY_PARAMETER');
      url.searchParams.append(key, value);
    }
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (payload !== undefined) headers['Content-Type'] = 'application/json';
    if (this.token && this.manifest.authHeader) {
      if (!/^X-[A-Za-z0-9-]+$/.test(this.manifest.authHeader) &&
          this.manifest.authHeader.toLowerCase() !== 'authorization') throw new Error('UNSAFE_AUTH_HEADER');
      headers[this.manifest.authHeader] = this.manifest.authHeader.toLowerCase() === 'authorization'
        ? 'Bearer ' + this.token : this.token;
    }
    // A response with status 204 only acknowledges transport, never native completion.
    const response = await fetch(url, {
      method, headers, body: payload === undefined ? undefined : JSON.stringify(payload),
      redirect: 'error', signal: AbortSignal.timeout(20000), cache: 'no-store',
    });
    const raw = await response.text();
    if (raw.length > 2_000_000) throw new Error('NATIVE_RESPONSE_TOO_LARGE');
    let body: unknown = null;
    if (raw.trim()) {
      try { body = JSON.parse(raw) as unknown; }
      catch { body = { unparsed: true }; }
    }
    return { status: response.status, body };
  }

  /** SSE streaming is live-only; lack of native replay MUST be reconciled separately. */
  async *stream(route: NativeRoute, replacements: Record<string, string> = {}, query: Record<string, string> = {},
    signal?: AbortSignal): AsyncIterable<{ event: string; id?: string; data: unknown }> {
    const path = this.manifest.paths[route];
    if (!path || !path.startsWith('/api/') || path.includes('..') || path.includes('?') ||
        path.includes('#')) throw new Error('NATIVE_STREAM_ROUTE_NOT_VERIFIED');
    let rendered = path;
    for (const [name, value] of Object.entries(replacements)) {
      if (!/^[a-zA-Z][a-zA-Z0-9]*$/.test(name) || !value || value.length > 512)
        throw new Error('INVALID_NATIVE_ROUTE_PARAMETER');
      rendered = rendered.replace('{' + name + '}', encodeURIComponent(value));
    }
    if (rendered.includes('{') || rendered.includes('}')) throw new Error('UNSAFE_NATIVE_ROUTE');
    const url = new URL(rendered, this.manifest.baseUrl);
    const base = new URL(this.manifest.baseUrl);
    if (url.origin !== base.origin) throw new Error('NATIVE_ORIGIN_MISMATCH');
    for (const [key, value] of Object.entries(query)) {
      if (!/^[a-zA-Z][a-zA-Z0-9.]*$/.test(key) || value.length > 4096)
        throw new Error('INVALID_NATIVE_QUERY_PARAMETER');
      url.searchParams.set(key, value);
    }
    const headers: Record<string, string> = { Accept: 'text/event-stream' };
    if (this.token && this.manifest.authHeader) {
      if (!/^X-[A-Za-z0-9-]+$/.test(this.manifest.authHeader) &&
          this.manifest.authHeader.toLowerCase() !== 'authorization') throw new Error('UNSAFE_AUTH_HEADER');
      headers[this.manifest.authHeader] = this.manifest.authHeader.toLowerCase() === 'authorization'
        ? 'Bearer ' + this.token : this.token;
    }
    const response = await fetch(url, { method: 'GET', headers, redirect: 'error', cache: 'no-store', signal });
    if (!response.ok || !response.body || !response.headers.get('content-type')?.includes('text/event-stream'))
      throw new Error('NATIVE_SSE_NOT_AVAILABLE: ' + response.status);
    let pending = '';
    const decoder = new TextDecoder();
    let size = 0;
    const reader = response.body.getReader();
    try {
      while (true) {
        const next = await reader.read();
        if (next.done) break;
        const chunk = next.value;
        pending += decoder.decode(chunk, { stream: true });
      size += chunk.length;
      if (pending.length > 1_000_000) throw new Error('NATIVE_SSE_FRAME_TOO_LARGE');
      // Reset aggregate byte counter on successfully split frames, not on every chunk.
      if (size > 50_000_000) throw new Error('NATIVE_SSE_BUDGET_EXCEEDED');
      let boundary: number;
      while ((boundary = pending.search(/\r?\n\r?\n/)) >= 0) {
        const separator = pending.match(/\r?\n\r?\n/)![0].length;
        const lines = pending.slice(0, boundary).split(/\r?\n/);
        pending = pending.slice(boundary + separator);
        const dataLines: string[] = [];
        let event = 'message', id: string | undefined;
        for (const line of lines) {
          if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart());
          else if (line.startsWith('event:')) event = line.slice(6).trim();
          else if (line.startsWith('id:')) id = line.slice(3).trim();
        }
        if (!dataLines.length) continue;
        const raw = dataLines.join('\n');
        let data: unknown;
        try { data = JSON.parse(raw) as unknown; }
        catch { throw new Error('NATIVE_SSE_INVALID_JSON'); }
        yield { event, id, data };
      }
      }
    } finally {
      await reader.cancel().catch(() => {});
      reader.releaseLock();
    }
  }
}

const SECTION_NAMES = [
  '评审摘要', '调用链与覆盖范围', '风险清单', '问题明细', '后续处理与评审边界',
] as const;

function assertInsideRoot(root: string, absolute: string): void {
  const rel = relative(root, absolute);
  if (rel === '' || rel === '..' || rel.startsWith('../') || rel.startsWith('..\\') || isAbsolute(rel))
    throw new Error('REPORT_PATH_OUTSIDE_SNAPSHOT');
}

export async function verifyWrittenReport(snapshotRoot: string, runStartedAt: number): Promise<{
  sha256: string; bytes: number; sections: number; path: string;
}> {
  const root = await realpath(snapshotRoot);
  const report = resolve(root, REPORT_RELATIVE_PATH);
  assertInsideRoot(root, report);
  const parent = await realpath(dirname(report));
  assertInsideRoot(root, parent);
  const info = await lstat(report);
  if (!info.isFile() || info.isSymbolicLink()) throw new Error('REPORT_NOT_REGULAR_FILE');
  const content = await readFile(report);
  if (content.length === 0 || content.length > 5_000_000 || info.mtimeMs < runStartedAt - 1000)
    throw new Error('REPORT_MISSING_OR_STALE');
  const markdown = content.toString('utf8');
  let position = -1;
  for (const section of SECTION_NAMES) {
    const idx = markdown.indexOf('## ' + section, position + 1);
    if (idx <= position) throw new Error('REPORT_FIVE_CHAPTERS_INVALID: ' + section);
    position = idx;
  }
  const freshInfo = await stat(report);
  if (freshInfo.size !== content.length || freshInfo.mtimeMs !== info.mtimeMs)
    throw new Error('REPORT_CONCURRENTLY_MODIFIED');
  return {
    sha256: createHash('sha256').update(content).digest('hex'),
    bytes: content.length, sections: SECTION_NAMES.length, path: REPORT_RELATIVE_PATH,
  };
}
