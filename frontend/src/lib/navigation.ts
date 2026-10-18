export function safeReturnPath(path: string | null): string {
  return path && path.startsWith("/") && !path.startsWith("//") && !path.includes("\\")
    ? path
    : "/dashboard";
}
