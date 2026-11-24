/**
 * OpenCode 状态映射
 *
 * 职责：将原生服务的返回字段转换为工作台内部领域结构。
 * Java 研发阅读提示：映射层不应编造状态，也不等同于执行操作本身。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
/** Input validation before the fixed native command. No Shell interpolation. */
const CONTROLLER = /^[A-Za-z_][A-Za-z0-9_]*$/;
export const REVIEW_BASE_ALIAS = 'harness-review-base';
export const REVIEW_COMMAND = 'codea-review';
export const AGENT_ID = 'codea-reviewer';
export const SKILL_ID = 'codea-review';
export const REPORT_RELATIVE_PATH = 'temp/codea-review-report.md';

export function reviewArguments(controller?: string): string {
  if (controller === undefined) return '--base ' + REVIEW_BASE_ALIAS;
  if (controller.length > 128 || !CONTROLLER.test(controller)) throw new Error('INVALID_CONTROLLER');
  return '--base ' + REVIEW_BASE_ALIAS + ' --controller ' + controller;
}

/** An upstream native ID cannot be treated as a Harness UUID. */
export function assertNativeId(value: unknown): string {
  if (typeof value !== 'string' || value.trim().length === 0 || value.length > 512)
    throw new Error('INVALID_NATIVE_ID');
  return value;
}
