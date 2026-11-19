import { appUrl, readBase, withBase } from "./base";

function doc(content?: string): Document {
  const d = document.implementation.createHTMLDocument("x");
  if (content !== undefined) {
    const m = d.createElement("meta");
    m.name = "base-path";
    m.content = content;
    d.head.append(m);
  }
  return d;
}

test("readBase reads the shell's meta, the root without one", () => {
  expect(readBase(doc())).toBe("");
  expect(readBase(doc(""))).toBe("");
  expect(readBase(doc("/reworm"))).toBe("/reworm");
  expect(readBase(doc("/reworm/"))).toBe("/reworm");
});

test("withBase prefixes app paths only", () => {
  expect(withBase("/api/me", "/reworm")).toBe("/reworm/api/me");
  expect(withBase("/api/me", "")).toBe("/api/me");
  expect(withBase("/api/me")).toBe("/api/me"); // tests run at the root
  for (const final of [
    "https://audio-ssl.itunes.apple.com/x.m4a",
    "//cdn.example/x",
    "data:audio/wav;base64,AA",
  ])
    expect(withBase(final, "/reworm")).toBe(final);
});

test("appUrl is the full address under the base", () => {
  expect(appUrl("/join/K7Q4", "/reworm")).toBe(
    `${window.location.origin}/reworm/join/K7Q4`,
  );
  expect(appUrl("https://games.example/reworm/pair/X", "/reworm")).toBe(
    "https://games.example/reworm/pair/X",
  );
});
