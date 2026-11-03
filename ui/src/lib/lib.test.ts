import { artGradient, hash, initials } from "./art";
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

  it("initials of the first two words", () => {
    expect(initials("Mark Ronson, Bruno Mars")).toBe("MR");
    expect(initials("R.E.M.")).toBe("R");
    expect(initials("(What's the Story) Morning Glory?")).toBe("WT");
    expect(initials("Édith Piaf")).toBe("ÉP");
    expect(initials("Die Ärzte")).toBe("DÄ");
    expect(initials("Joseíto Fernández")).toBe("JF");
    expect(initials("& more")).toBe("M");
  });
});

describe("cx", () => {
  it("joins the truthy names", () => {
    expect(cx("a", false, null, undefined, "", "b")).toBe("a b");
  });
});
