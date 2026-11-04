import { useState } from "react";
import type { Deck, Decks } from "../api/types";
import { Chip, Segmented } from "../components/Controls";
import { cx } from "../lib/cx";

type Tab = "sets" | "categories" | "genres";

const HINT: Record<Tab, string> = {
  sets: "Card lists matched to Apple Music. The year is the one printed on the card. Pick several to mix them, like at game night.",
  categories:
    'Well-known songs only: a song counts if it\'s on most of the "best of the 60s / 80s / 2010s …" playlists for its decade.',
  genres:
    "General = everything, the widest span. Pick one or more genres to narrow it down.",
};

const toggle = (xs: string[], x: string) =>
  xs.includes(x) ? xs.filter((y) => y !== x) : [...xs, x];

// DeckPicker: card sets, categories and genres, several at once (training setup, party lobby).
export function DeckPicker({
  decks,
  deck,
  onChange,
  hints = true,
}: {
  decks: Decks;
  deck: Deck;
  onChange: (d: Deck) => void;
  hints?: boolean;
}) {
  const [tab, setTab] = useState<Tab>(
    deck.sets.length || (!deck.categories.length && !deck.genres.length)
      ? "sets"
      : deck.categories.length
        ? "categories"
        : "genres",
  );
  const pick = (k: Tab, id: string) =>
    onChange({ ...deck, [k]: toggle(deck[k], id) });
  return (
    <>
      <Segmented
        label="What to listen to"
        value={tab}
        onChange={setTab}
        options={[
          { value: "sets", label: "Sets" },
          { value: "categories", label: "Categories" },
          { value: "genres", label: "Genres" },
        ]}
      />
      <div style={{ marginTop: 12 }}>
        {tab === "sets" && (
          <div className="sets">
            {decks.sets.map((s) => (
              <button
                key={s.id}
                type="button"
                className={cx("opt", deck.sets.includes(s.id) && "on")}
                aria-pressed={deck.sets.includes(s.id)}
                onClick={() => pick("sets", s.id)}
              >
                <div className="t">{s.name}</div>
                <div className="s num">
                  {s.count} · {s.from}–{s.to}
                </div>
              </button>
            ))}
          </div>
        )}
        {tab !== "sets" && (
          <div className="chips">
            {decks[tab].map((c) => (
              <Chip
                key={c.id}
                on={deck[tab].includes(c.id)}
                count={c.count}
                onClick={() => pick(tab, c.id)}
              >
                {c.name}
              </Chip>
            ))}
          </div>
        )}
        {hints && (
          <div className="sub" style={{ marginTop: 10, fontSize: 12.5 }}>
            {HINT[tab]}
          </div>
        )}
      </div>
    </>
  );
}
