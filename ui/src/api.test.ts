import { fetchVersion } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchVersion", () => {
  it("returns the version", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ version: "dev-abc1234" })),
    );
    await expect(fetchVersion()).resolves.toBe("dev-abc1234");
  });

  it("fails on an error status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("down", { status: 503 })),
    );
    await expect(fetchVersion()).rejects.toThrow("503");
  });

  it("fails on a body without a version", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({})),
    );
    await expect(fetchVersion()).rejects.toThrow("no version");
  });
});
