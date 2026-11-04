import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DeckArt, deckArt } from "./DeckArt";

const note = deckArt("x", "Other").glyph;

describe("deckArt", () => {
  it("gives every card set its own picture, by slug", () => {
    const sets = [
      "essentials",
      "mega-mix",
      "rock-anthems",
      "schlager-fest",
      "sunshine",
      "cheesy-classics",
      "party-starters",
      "crowd-pleasers",
      "screen-hits",
      "christmas-hits",
      "radio-gold",
    ];
    const glyphs = new Set(sets.map((s) => deckArt(s, "").glyph));
    expect(glyphs.size).toBe(sets.length);
    expect(glyphs.has(note)).toBe(false);
  });

  it("keys on the slug, not the display name", () => {
    expect(deckArt("rock-anthems", "Renamed").glyph).toBe(
      deckArt("rock-anthems", "Rock Anthems").glyph,
    );
  });

  it("guesses unknown decks from whole words of the name", () => {
    expect(deckArt("x", "Summer nights").glyph).toBe(
      deckArt("summer", "").glyph,
    );
    expect(deckArt("x", "Movie classics").glyph).toBe(
      deckArt("movie-tv", "").glyph,
    );
    for (const n of ["Phones", "Stardust", "Radiohead", "Steampunk", "Clover"])
      expect(deckArt("x", n).glyph).toBe(note);
  });

  it("falls back to a note on rotating colours, six apart", () => {
    const a = deckArt("x", "Something else", 0);
    expect(deckArt("x", "Something else", 1).from).not.toBe(a.from);
    expect(deckArt("x", "Something else", 4).from).not.toBe(a.from);
    expect(deckArt("x", "Something else", 6).from).toBe(a.from);
  });

  it("renders no letters", () => {
    const { container } = render(
      <DeckArt id="rock-anthems" name="Rock Anthems" />,
    );
    expect(container.textContent).toBe("");
    expect(container.querySelectorAll("svg")).toHaveLength(2);
  });
});
