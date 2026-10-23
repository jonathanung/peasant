import { getApiBaseUrl } from './base';

/** The states GET /api/v1/sync/ingest/status reports. */
export type IngestRunStatus = 'idle' | 'running' | 'done' | 'error';

const INGEST_PATH = '/api/v1/sync/ingest';
const INGEST_STATUS_PATH = '/api/v1/sync/ingest/status';

/**
 * Re-index this computer's sessions through the local server's own ingest run
 * (POST /api/v1/sync/ingest), and wait for it to finish. A run already in
 * progress is waited on rather than started twice. It reads the recorded
 * sessions and indexes them here; nothing leaves the machine.
 */
export async function reindexAndWait({ signal, pollMs = 1000 }: { signal?: AbortSignal; pollMs?: number } = {}): Promise<void> {
  const base = getApiBaseUrl();
  const start = await fetch(`${base}${INGEST_PATH}`, { method: 'POST', signal });
  if (!start.ok && start.status !== 409) {
    throw new Error(`the local index could not start (${start.status})`);
  }
  for (;;) {
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(resolve, pollMs);
      signal?.addEventListener('abort', () => { clearTimeout(timer); reject(signal.reason); }, { once: true });
    });
    const response = await fetch(`${base}${INGEST_STATUS_PATH}`, { signal });
    if (!response.ok) throw new Error(`the local index status could not be read (${response.status})`);
    const body = (await response.json()) as { status?: IngestRunStatus; error?: string };
    if (body.status === 'done') return;
    if (body.status === 'error') throw new Error(body.error ?? 'the local index failed');
  }
}
