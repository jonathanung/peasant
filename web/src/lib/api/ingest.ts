import { getApiBaseUrl } from './base';
import type { ReindexFailureCode } from '@/lib/share/scanFailure';

const SESSION_INGEST_PATH = '/api/v1/sync/ingest/session';

/**
 * The server stops a session re-index after two minutes and says so; the
 * client waits a little longer, so a server that went away (a restart, a
 * crash) still ends the wait with a reason instead of a spinner.
 */
export const SESSION_REINDEX_TIMEOUT_MS = 150_000;

/** A re-index that did not finish: why, as a code the popup words, and the whole reason. */
export class SessionReindexError extends Error {
  constructor(readonly code: ReindexFailureCode, message: string) {
    super(message);
    this.name = 'SessionReindexError';
  }
}

const SERVER_CODES: Record<string, ReindexFailureCode> = {
  ingest_running: 'ingest_running',
  session_ingest_timeout: 'session_ingest_timeout',
  session_not_recorded: 'session_not_recorded',
};

/** A finished re-index: what the run could not read, in its own words. */
export interface SessionReindex {
  notes: string[];
}

/**
 * Re-index one recorded session through the local server
 * (POST /api/v1/sync/ingest/session) and wait for its answer. It re-reads this
 * session's recording only, on this computer; nothing leaves the machine. A
 * refusal, a timeout or a server that stopped answering throws one line that
 * names what happened and how to retry.
 */
export async function reindexSession(
  sessionId: string,
  { signal, timeoutMs = SESSION_REINDEX_TIMEOUT_MS }: { signal?: AbortSignal; timeoutMs?: number } = {},
): Promise<SessionReindex> {
  const controller = new AbortController();
  let timedOut = false;
  const timer = setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs);
  const forward = () => controller.abort(signal?.reason);
  signal?.addEventListener('abort', forward, { once: true });
  try {
    let response: Response;
    try {
      const params = new URLSearchParams({ session_id: sessionId });
      response = await fetch(`${getApiBaseUrl()}${SESSION_INGEST_PATH}?${params.toString()}`, { method: 'POST', signal: controller.signal });
    } catch (error) {
      if (signal?.aborted) throw error;
      if (timedOut) throw new SessionReindexError('session_ingest_timeout', `updating this session’s index took longer than ${Math.round(timeoutMs / 1000)}s and was stopped; retry`);
      throw new SessionReindexError('unreachable', 'the peasant app on this computer stopped answering while updating this session’s index; retry');
    }
    const body = (await response.json().catch(() => ({}))) as { error?: unknown; code?: unknown; notes?: unknown };
    if (!response.ok) {
      const code = typeof body.code === 'string' ? SERVER_CODES[body.code] ?? 'other' : 'other';
      throw new SessionReindexError(code, typeof body.error === 'string' && body.error.trim() ? body.error.trim() : `updating this session’s index failed (${response.status}); retry`);
    }
    const notes = Array.isArray(body.notes) ? body.notes.filter((note): note is string => typeof note === 'string' && note.trim() !== '') : [];
    return { notes };
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener('abort', forward);
  }
}
