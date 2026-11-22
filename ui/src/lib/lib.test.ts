import { artGradient, hash } from "./art";
import { cx } from "./cx";
import type { Deck } from "../api/types";
import { allYears, loadSettings, save } from "./prefs";

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

describe("loadSettings", () => {
  const deck = (d: Partial<Deck>) => allYears(d);
  it("keeps a saved deck to the kind the picker opens on", () => {
    save("train", {
      deck: deck({ sets: ["mega"], categories: ["gen-alpha"] }),
    });
    expect(loadSettings<{ deck: Deck }>("train")!.deck).toEqual(
      deck({ sets: ["mega"] }),
    );
    save("room", {
      toWin: 5,
      deck: deck({ categories: ["gen-alpha"], genres: ["pop"] }),
    });
    expect(loadSettings<{ deck: Deck; toWin: number }>("room")).toEqual({
      toWin: 5,
      deck: deck({ categories: ["gen-alpha"] }),
    });
    save("room", { deck: deck({ genres: ["pop"] }) });
    expect(loadSettings<{ deck: Deck }>("room")!.deck).toEqual(
      deck({ genres: ["pop"] }),
    );
    save("room", null);
    expect(loadSettings("room")).toBeNull();
  });
});
