import { artGradient, hash } from "./art";
import { cx } from "./cx";

describe("art", () => {
  it("hashes stably and draws a gradient from it", () => {
    expect(hash("")).toBe(7);
    expect(hash("ab")).toBe(hash("ab"));
    expect(hash("ab")).not.toBe(hash("ba"));
    expect(artGradient("Dancing QueenABBA")).toMatch(
      /^linear-gradient\(135deg,hsl\(\d+ 62% 52%\),hsl\(\d+ 55% 28%\)\)$/,
    );
  });
});

describe("cx", () => {
  it("joins the truthy names", () => {
    expect(cx("a", false, null, undefined, "", "b")).toBe("a b");
  });
});
