// A temporary shim: the shared rule for a session title's display lives in
// fairtrade's title helper (summarizePrompt / composeSessionTitle in
// @peasant-labs/fairtrade/ui). That helper does not yet treat Cursor's
// <user_query> wrapper as a title wrapper, so until it does this module drops
// it here. Delete it, and call the helper, once the pinned fairtrade does.
//
// Cursor wraps every prompt in <user_query>…</user_query>. A title derived from
// such a prompt keeps the wrapper; the words inside are what the person typed.
const USER_QUERY_OPEN = '<user_query>';
const USER_QUERY_CLOSE = '</user_query>';

/** The title's words without Cursor's prompt wrapper, whitespace collapsed. */
export function cleanSessionTitle(text: string): string {
  let inner = text;
  const start = inner.indexOf(USER_QUERY_OPEN);
  if (start !== -1) {
    inner = inner.slice(start + USER_QUERY_OPEN.length);
    const end = inner.indexOf(USER_QUERY_CLOSE);
    if (end !== -1) inner = inner.slice(0, end);
  }
  return inner.replace(/\s+/g, ' ').trim();
}
