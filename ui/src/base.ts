// The path prefix the app is served under ("/reworm"), "" at the host's root. The server writes it into the shell's
// base-path meta (PUBLIC_PATH); without the meta (tests, the vite dev server) the app sits at the root.
export function readBase(doc: Document = document): string {
  const v = doc
    .querySelector('meta[name="base-path"]')
    ?.getAttribute("content");
  return (v ?? "").replace(/\/+$/, "");
}

export const BASE = readBase();

// withBase prefixes an app-relative path ("/api/rooms", "/join/K7Q4"; what the API hands out too) with the base.
// Anything else (an absolute URL, a protocol-relative one, data:) is final and passes untouched.
export function withBase(path: string, base: string = BASE): string {
  return path.startsWith("/") && !path.startsWith("//") ? base + path : path;
}

// appUrl is the full address of an app path, for links that leave this browser (QR codes).
export function appUrl(path: string, base: string = BASE): string {
  return new URL(withBase(path, base), window.location.origin).href;
}
