import type { NextConfig } from "next";

// Security headers for every page. A full script/style CSP is deliberately
// NOT set: Monaco loads its editor and workers from a CDN via blob: workers
// and the layout ships an inline no-flash theme script, so a strict policy
// needs nonces/hashes wired through the app (future work). What is set here
// is safe for this app as-is:
//   - frame-ancestors / X-Frame-Options: no clickjacking via iframes;
//   - object-src 'none', base-uri 'self': no plugin content, no <base> hijack;
//   - nosniff, a privacy-preserving Referrer-Policy, and a Permissions-Policy
//     that turns off device APIs the app never uses.
const securityHeaders = [
  {
    key: "Content-Security-Policy",
    value: "frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
  },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Permissions-Policy",
    value: "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
  },
];

const nextConfig: NextConfig = {
  poweredByHeader: false,
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
