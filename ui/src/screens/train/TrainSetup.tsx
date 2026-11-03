// /train — a new training session: what to listen to, the years, the mode, difficulty, clip and length.
import { useState } from "react";
import { useApi, useAsync, useMe } from "../../api/context";
import type { ClipStart, Difficulty, TrainSettings } from "../../api/types";
import { Button, Segmented } from "../../components/Controls";
import { Icon } from "../../components/Icon";
import { cx } from "../../lib/cx";
import { lastTraining } from "../Home";
import {
  Distrib,
  Loading,
  Problem,
  Screen,
  ScreenHead,
  Unpaired,
  YearRange,
} from "../common";
import { DeckPicker } from "../DeckPicker";
import { useStartTraining } from "./useStart";

const MODES = [
  {
    k: "years",
    ic: "⇄",
    t: "Earlier · Spot on · Later",
    s: "A year is shown. Is the song earlier, later — or is it exactly that year?",
  },
  {
    k: "pin",
    ic: "◎",
    t: "Pin the year",
    s: "Dial in the year yourself. ±1 counts, exact earns a bonus.",
  },
  {
    k: "line",
    ic: "☰",
    t: "Timeline",
    s: "Place each song between the ones you already got right — like game night.",
    soon: true,
  },
] as const;

export function TrainSetup() {
  const api = useApi();
  const { me, unpaired } = useMe();
  const decks = useAsync(me ? "decks" : null, () => api.decks());
  const [cfg, setCfg] = useState<TrainSettings | null>(null);
  const training = useStartTraining();

  if (unpaired)
    return (
      <Screen tab="train">
        <ScreenHead back="/" title="New session" />
        <Unpaired what="Training" />
      </Screen>
    );
  if (decks.error)
    return (
      <Screen tab="train">
        <ScreenHead back="/" title="New session" />
        <Problem error={decks.error} retry={decks.reload} />
      </Screen>
    );
  if (!decks.data)
    return (
      <Screen tab="train">
        <ScreenHead back="/" title="New session" />
        <Loading />
      </Screen>
    );

  const c = cfg ?? lastTraining(decks.data);
  const set = (patch: Partial<TrainSettings>) => setCfg({ ...c, ...patch });

  return (
    <Screen
      footer={
        <Button
          icon="play"
          disabled={training.busy}
          onClick={() => training.start(c)}
        >
          Start · {c.length} songs
        </Button>
      }
    >
      <ScreenHead back="/" title="New session" />
      <div className="cols">
        <div className="col">
          <div className="label">What to listen to</div>
          <DeckPicker
            decks={decks.data}
            deck={c.deck}
            onChange={(deck) => set({ deck })}
          />
          <div className="label row between">
            <span>Release years</span>
            <span className="num paper" style={{ letterSpacing: 0 }}>
              {c.deck.from} – {c.deck.to}
            </span>
          </div>
          <YearRange
            from={c.deck.from}
            to={c.deck.to}
            onChange={(from, to) => set({ deck: { ...c.deck, from, to } })}
          />
        </div>
        <div className="col">
          <div className="label">Mode</div>
          {MODES.map((m) => (
            <button
              key={m.k}
              type="button"
              className={cx("opt", c.mode === m.k && "on")}
              aria-pressed={c.mode === m.k}
              disabled={"soon" in m}
              onClick={() => m.k !== "line" && set({ mode: m.k })}
            >
              <div className="ic" aria-hidden="true">
                {m.ic}
              </div>
              <div>
                <div className="t">
                  {m.t}
                  {"soon" in m && <span className="tag">soon</span>}
                </div>
                <div className="s">{m.s}</div>
              </div>
            </button>
          ))}
          <div className="label">Difficulty</div>
          <Segmented<Difficulty>
            label="Difficulty"
            value={c.difficulty}
            onChange={(difficulty) => set({ difficulty })}
            options={[
              { value: "easy", label: "Easy" },
              { value: "medium", label: "Medium" },
              { value: "hard", label: "Hard" },
            ]}
          />
          {c.mode === "years" && <Distrib difficulty={c.difficulty} />}
          <div className="label">Clip</div>
          <Segmented<ClipStart>
            label="Clip start"
            value={c.clip.start}
            onChange={(start) => set({ clip: { ...c.clip, start } })}
            options={[
              { value: "chorus", label: "Before the chorus" },
              { value: "random", label: "Random spot" },
            ]}
          />
          <div style={{ marginTop: 8 }}>
            <Segmented
              label="Clip length"
              value={String(c.clip.length)}
              onChange={(v) => set({ clip: { ...c.clip, length: +v } })}
              options={[10, 20, 30, 60].map((n) => ({
                value: String(n),
                label: `${n} s`,
              }))}
            />
          </div>
          <div className="label">Session length</div>
          <Segmented
            label="Session length"
            value={String(c.length)}
            onChange={(v) => set({ length: +v })}
            options={[10, 20, 40].map((n) => ({
              value: String(n),
              label: `${n} songs`,
            }))}
          />
          {training.error && <Problem error={training.error} />}
          <div className="sub" style={{ fontSize: 12, marginTop: 14 }}>
            <Icon name="flame" size={14} /> Missed songs come back a few cards
            later with a different year.
          </div>
        </div>
      </div>
      <div style={{ height: 10 }} />
    </Screen>
  );
}
