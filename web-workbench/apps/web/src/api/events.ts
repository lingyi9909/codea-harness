export type LiveEvent = { taskId: string; runId: string | null; seq: string; type: string; occurredAt: string; data: unknown };
/** EventSource auto-reconnects, while API snapshots remain the source of truth. */
export function subscribeTask(taskId: string, after: string, onUpdate: (value: LiveEvent) => void,
  onError: () => void): () => void {
  const target = '/api/v1/tasks/' + encodeURIComponent(taskId) + '/events?after=' + encodeURIComponent(after);
  const stream = new EventSource(target, { withCredentials: true });
  stream.onmessage = event => {
    try {
      const item = JSON.parse(event.data) as LiveEvent;
      if (item.taskId === taskId && /^\d+$/.test(item.seq)) onUpdate(item);
    } catch { onError(); }
  };
  stream.onerror = () => onError();
  return () => stream.close();
}
