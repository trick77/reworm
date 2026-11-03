// fetchVersion is the server's build version (GET /api/version).
export async function fetchVersion(signal?: AbortSignal): Promise<string> {
  const res = await fetch("/api/version", { signal });
  if (!res.ok) {
    throw new Error(`GET /api/version: ${res.status}`);
  }
  const body = (await res.json()) as { version?: unknown };
  if (typeof body.version !== "string") {
    throw new Error("GET /api/version: no version in the response");
  }
  return body.version;
}
