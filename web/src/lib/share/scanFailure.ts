/**
 * What a failed redaction scan means to the person publishing. A stale local
 * index is healed by the popup's re-scan (it re-indexes, then scans again), so
 * it gets one short line. Any other failure keeps its actionable message, the what, why
 * and fix the redaction invariants require to stay visible, without the
 * transport wrapper: no HTTP status, no JSON envelope.
 */
export type ScanFailureKind = 'needs-reindex' | 'other';

export interface ScanFailure {
  kind: ScanFailureKind;
  /** The one line the dialog's failure box shows. */
  line: string;
  /** The text as it arrived, for a details view only. */
  detail: string;
}

/**
 * The local index, not the recording, is at fault, and re-indexing rewrites
 * it: the stored session holds evidence the strict publication reader refuses
 * (ingest.EvidenceIntegrityError and LegacyPositionUnavailableError share the
 * first prefix), or its stored capture is not a verified one yet (the store's
 * publication input refusal names the second).
 */
const STALE_INDEX = ['retain unknown source evidence in ingest', 'publication metadata needs ingest'];

/** fetchRedactionScan's wrapper around a refused GET /api/v1/sync/redactions. */
const TRANSPORT = /^redaction scan failed \(\d+\):\s*/;

/** The server's own message, out of its `{"error": ...}` envelope when it has one. */
function unwrap(detail: string): string {
  const body = detail.replace(TRANSPORT, '').trim();
  try {
    const parsed = JSON.parse(body) as { error?: unknown };
    if (parsed && typeof parsed.error === 'string' && parsed.error.trim()) return parsed.error.trim();
  } catch {
    // not JSON: the message is the body itself
  }
  return body || 're-scan to try again.';
}

export function classifyScanFailure(detail: string): ScanFailure {
  if (STALE_INDEX.some((marker) => detail.includes(marker))) {
    return { kind: 'needs-reindex', line: 'this session needs a quick re-index before it can be published.', detail };
  }
  return { kind: 'other', line: `the scan failed, so publish is off. ${unwrap(detail)}`, detail };
}
