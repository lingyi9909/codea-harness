import { createHash } from 'node:crypto';
import { lstat, readFile, realpath, stat } from 'node:fs/promises';
import { dirname, relative, resolve, isAbsolute } from 'node:path';
import { REPORT_RELATIVE_PATH } from './mapping.ts';

export type NativeMethod = 'GET' | 'POST' | 'PUT';
export type NativeRoute = 'identity' | 'createSession' | 'session' | 'command'
  | 'messages' | 'events' | 'state' | 'permissions' | 'permissionReply'
  | 'forms' | 'formReply' | 'prompt' | 'interrupt' | 'instructions' | 'log';
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
  private readonly token?: string;
  constructor(manifest: NativeWireManifest, token?: string) {
    this.manifest = manifest;
    this.token = token;
    const url = new URL(manifest.baseUrl);
    if (url.protocol !== 'http:' ||
        !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname) ||
        url.username || url.password || url.search || url.hash)
      throw new Error('NATIVE_ENDPOINT_NOT_LOOPBACK');
    if (!manifest.build.trim() || !manifest.serviceIdentity.trim()) throw new Error('NATIVE_IDENTITY_NOT_PINNED');
  }

  async call(route: NativeRoute, method: NativeMethod, payload?: unknown,
    replacements: Record<string, string> = {}): Promise<{ status: number; body: unknown }> {
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
