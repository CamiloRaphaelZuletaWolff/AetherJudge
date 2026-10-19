const DEFAULT_RETURN_PATH = "/dashboard";

// Placeholder origin used only to resolve the candidate path; any result
// with a different origin means the "path" would leave this site.
const PROBE_ORIGIN = "http://same-origin.invalid";

// safeReturnPath validates the post-login `?next=` target so it can only
// point back into this app (no open redirect). Browsers strip tabs/newlines
// and treat "\" like "/" while parsing URLs, so "/\t/evil.com" or "/\evil.com"
// would otherwise become the protocol-relative "//evil.com". Anything that is
// not a plain same-origin path falls back to the dashboard.
export function safeReturnPath(path: string | null): string {
  if (!path || !path.startsWith("/") || path.startsWith("//") || /[\\\s]/.test(path)) {
    return DEFAULT_RETURN_PATH;
  }
  for (const ch of path) {
    const code = ch.charCodeAt(0);
    if (code < 0x20 || code === 0x7f) return DEFAULT_RETURN_PATH;
  }
  try {
    if (new URL(path, PROBE_ORIGIN).origin !== PROBE_ORIGIN) return DEFAULT_RETURN_PATH;
  } catch {
    return DEFAULT_RETURN_PATH;
  }
  return path;
}
