import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DeckArt, deckArt } from "./DeckArt";

describe("deckArt", () => {
  it("gives every card set its own picture", () => {
    const sets = [
      "Essentials",
      "Mega Mix",
      "Rock Anthems",
      "Schlager Fest",
      "Sunshine",
      "Cheesy Classics",
      "Party Starters",
      "Crowd Pleasers",
      "Screen Hits",
      "Christmas Hits",
      "Radio Gold",
    ];
    const glyphs = new Set(sets.map((s) => deckArt(s).glyph));
    expect(glyphs.size).toBe(sets.length);
  });

  it("falls back to a note on rotating colours", () => {
    const a = deckArt("Something else", 0);
    const b = deckArt("Something else", 1);
    expect(a.glyph).toBe(b.glyph);
    expect(a.from).not.toBe(b.from);
  });

  it("does not match words inside words", () => {
    expect(deckArt("Phones").glyph).toBe(deckArt("Other").glyph);
    expect(deckArt("Smash Hits").glyph).not.toBe(deckArt("Other").glyph);
  });

  it("renders no letters", () => {
    const { container } = render(<DeckArt name="Rock Anthems" />);
    expect(container.textContent).toBe("");
    expect(container.querySelectorAll("svg")).toHaveLength(2);
  });
});
