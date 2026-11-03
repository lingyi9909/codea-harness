// Dependency-free OpenAPI schema DTO generator for the checked-in design YAML.
// T01 scope: components.schemas; HTTP operation clients belong to later Tasks.
import { readFileSync, writeFileSync } from 'node:fs';

const yaml = readFileSync(new URL('../../docs/superpowers/specs/contracts/2026-11-02-code-harness-web-v1.openapi.yaml', import.meta.url), 'utf8');
const all = yaml.split(/\r?\n/);
const start = all.findIndex(line => line === '  schemas:');
if (start < 0) throw new Error('OPENAPI_SCHEMA_BLOCK_MISSING');

const lines = all.slice(start + 1)
  .map(t => ({ indent: t.length - t.trimStart().length, s: t.trim() }))
  .filter(v => v.s && !v.s.startsWith('#'));

// These anchors are declared above components.schemas in the original OpenAPI.
// Preserve their exact semantic kinds when parsing this subtree.
const anchors = new Map(Object.entries({
  id004: { type: 'string' },
  id005: { type: 'string' },
  id010: ['QUEUED', 'PREPARING', 'RUNNING', 'WAITING_INPUT', 'WAITING_PERMISSION',
    'COLLECTING', 'STOP_REQUESTED', 'RECOVERING', 'SUCCEEDED', 'FAILED', 'STOPPED'],
  id011: { type: 'string' },
}));
function scalar(input) {
  const v = input.trim();
  if (v.startsWith('*')) {
    if (!anchors.has(v.slice(1))) throw new Error('UNKNOWN_YAML_ALIAS: ' + v);
    return anchors.get(v.slice(1));
  }
  if (v.startsWith("'") && v.endsWith("'")) return v.slice(1, -1);
  if (v === 'true') return true;
  if (v === 'false') return false;
  if (v === 'null') return null;
  return v;
}
const nested = (i, depth) => i + 1 < lines.length &&
  (lines[i + 1].indent > depth ||
   (lines[i + 1].indent === depth && lines[i + 1].s.startsWith('- ')));
function parse(i, depth) {
  if (lines[i]?.indent === depth && lines[i].s.startsWith('- ')) {
    const result = [];
    while (i < lines.length && lines[i].indent === depth && lines[i].s.startsWith('- ')) {
      const tail = lines[i].s.slice(2);
      let value;
      if (/^[^:]+: /.test(tail)) {
        const [key, ...rest] = tail.split(': ');
        value = { [key]: scalar(rest.join(': ')) };
        if (i + 1 < lines.length && lines[i + 1].indent > depth) {
          const p = parse(i + 1, lines[i + 1].indent);
          Object.assign(value, p.value);
          i = p.next - 1;
        }
      } else if (tail.startsWith('&')) {
        const anchor = tail.slice(1);
        if (nested(i, depth)) {
          const p = parse(i + 1, lines[i + 1].indent);
          value = p.value;
          i = p.next - 1;
        } else value = null;
        anchors.set(anchor, value);
      } else if (!tail) {
        if (nested(i, depth)) {
          const p = parse(i + 1, lines[i + 1].indent);
          value = p.value;
          i = p.next - 1;
        } else value = null;
      } else value = scalar(tail);
      result.push(value);
      i++;
    }
    return { value: result, next: i };
  }
  const result = {};
  while (i < lines.length && lines[i].indent === depth && !lines[i].s.startsWith('- ')) {
    const match = lines[i].s.match(/^([^:]+):(?:\s+(.*))?$/);
    if (!match) throw new Error('UNSUPPORTED_YAML_LINE: ' + lines[i].s);
    const key = match[1];
    let v = match[2] ?? '';
    let anchor = null;
    if (v.startsWith('&')) { anchor = v.slice(1); v = ''; }
    if (v) result[key] = scalar(v);
    else if (nested(i, depth)) {
      const p = parse(i + 1, lines[i + 1].indent);
      result[key] = p.value;
      i = p.next - 1;
    } else result[key] = null;
    if (anchor) anchors.set(anchor, result[key]);
    i++;
  }
  return { value: result, next: i };
}
const { value: schemas, next } = parse(0, 4);
if (next !== lines.length || Object.keys(schemas).length < 60)
  throw new Error('SCHEMA_PARSE_INCOMPLETE');
const quote = value => JSON.stringify(String(value));
function emit(schema) {
  if (!schema || typeof schema !== 'object') return 'unknown';
  if (schema.$ref) return "components['schemas'][" + quote(schema.$ref.split('/').at(-1)) + ']';
  if (Array.isArray(schema.enum)) return schema.enum.map(quote).join(' | ') || 'never';
  if (schema.oneOf || schema.anyOf) return '(' + (schema.oneOf || schema.anyOf).map(emit).join(' | ') + ')';
  if (schema.allOf) return '(' + schema.allOf.map(emit).join(' & ') + ')';
  if (schema.type === 'array') return 'Array<' + emit(schema.items) + '>';
  if (schema.type === 'object' || schema.properties || schema.additionalProperties) {
    const props = schema.properties || {};
    const required = new Set(schema.required || []);
    const fields = Object.entries(props).map(([key, val]) =>
      '  ' + quote(key) + (required.has(key) ? '' : '?') + ': ' + emit(val) + ';');
    if (schema.additionalProperties === true) fields.push('  [key: string]: unknown;');
    else if (schema.additionalProperties && typeof schema.additionalProperties === 'object' && !fields.length)
      return 'Record<string, ' + emit(schema.additionalProperties) + '>';
    return fields.length ? '{\n' + fields.join('\n') + '\n}' :
      (schema.additionalProperties === false ? 'Record<string, never>' : 'Record<string, unknown>');
  }
  return ({ string: 'string', integer: 'number', number: 'number',
    boolean: 'boolean', null: 'null' })[schema.type] || 'unknown';
}
const generated = [
  '// @generated from docs/superpowers/specs/contracts/2026-11-02-code-harness-web-v1.openapi.yaml',
  '// Schema DTOs only; generated by web-workbench/scripts/generate-dto.mjs. Never edit by hand.',
  'export interface components {',
  '  schemas: {',
  ...Object.entries(schemas).map(([key, value]) =>
    '    ' + quote(key) + ': ' + emit(value).replace(/\n/g, '\n    ') + ';'),
  '  };',
  '}',
  '',
].join('\n');
writeFileSync(new URL('../packages/contracts/src/api.generated.ts', import.meta.url), generated, 'utf8');
console.log('Generated ' + Object.keys(schemas).length + ' OpenAPI schema DTOs');
