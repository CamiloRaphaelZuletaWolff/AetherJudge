// slugify derives a URL-safe contest slug from a title, mirroring the backend
// (slugify in internal/api/handlers_admin.go): lowercase, runs of
// non-alphanumerics collapsed to single hyphens, ends trimmed, clamped to 64.
// The backend re-validates, so this is a UX convenience, not the authority.
export function slugify(title: string): string {
  return title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64)
    .replace(/-+$/g, "");
}
