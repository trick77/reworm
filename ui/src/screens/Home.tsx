import type { ReactNode } from "react";
import { useLocation } from "wouter";
import { useApi, useAsync, useMe } from "../api/context";
import type { Decks, TrainSettings } from "../api/types";
import { avatarInfo } from "../avatars";
import { Avatar } from "../components/Avatar";
import { Button, Card, IconButton } from "../components/Controls";
import { allYears, defaultTraining, load } from "../lib/prefs";
import { Brand, deckLabel, Problem, Screen, SetupPending } from "./common";
import { useStartTraining } from "./train/useStart";

// lastTraining is the saved training settings, else the first card set.
export function lastTraining(decks?: Decks): TrainSettings {
  return (
    load<TrainSettings>("train") ??
    defaultTraining(
      allYears({ sets: decks?.sets[0] ? [decks.sets[0].id] : [] }),
    )
  );
}

type Action = { icon: "party" | "qr"; to: string; long: string; short: string };
const HOST: Action = {
  icon: "party",
  to: "/host",
  long: "Host a game",
  short: "Host",
};
const JOIN: Action = {
  icon: "qr",
  to: "/join",
  long: "Join a game",
  short: "Join",
};

// STEPS: a round of game night, the same for a guest and a host. The buzzer mode is the lobby's to explain.
const STEPS: ReactNode[] = [
  "One screen plays a song clip and shows a year.",
  <>
    Everyone taps <b>Earlier</b>, <b>Spot on</b> or <b>Later</b> on their phone.
  </>,
  "Fastest right answer takes the round.",
];

// GameNight leads with Join on a guest's phone (no profile, can only join) and with Host on a paired device.
// guest null: not known yet, no buttons, so a guest never taps a Host that turns into Join under the finger; also
// a fresh install, where no game can exist.
function GameNight({ guest }: { guest: boolean | null }) {
  const [, go] = useLocation();
  const [first, second] = guest ? [JOIN, HOST] : [HOST, JOIN];
  return (
    <Card className="hero" style={{ marginTop: 18, padding: 18 }}>
      <div className="kata">Game night</div>
      <div
        style={{
          fontSize: 24,
          fontWeight: 750,
          margin: "6px 0 2px",
          letterSpacing: -0.3,
        }}
      >
        Play with friends
      </div>
      <ol className="round-steps">
        {STEPS.map((s, i) => (
          <li key={i}>
            <span className="n" aria-hidden="true">
              {i + 1}
            </span>
            <span>{s}</span>
          </li>
        ))}
      </ol>
      {guest != null && (
        <div className="row" style={{ marginTop: 16, gap: 8 }}>
          <Button
            icon={first.icon}
            style={{ flex: 2 }}
            onClick={() => go(first.to)}
          >
            {first.long}
          </Button>
          <Button
            variant="ghost"
            icon={second.icon}
            style={{ flex: 1 }}
            onClick={() => go(second.to)}
          >
            {second.short}
          </Button>
        </div>
      )}
    </Card>
  );
}

// Home leads with game night; a paired device gets one compact card to train solo, the rest is the Train tab's.
export function Home() {
  const api = useApi();
  const [, go] = useLocation();
  const { me, unpaired, fresh, loading, error: meError, reload } = useMe();
  const decks = useAsync(me ? "decks" : null, () => api.decks());
  const training = useStartTraining();
  const today = lastTraining(decks.data);

  return (
    <Screen tab="home">
      <div className="home">
        <div className="row between" style={{ marginTop: 4 }}>
          <Brand />
          {me && (
            <button
              type="button"
              className="me-btn"
              title="Profile & devices"
              aria-label="Profile & devices"
              onClick={() => go("/me")}
              style={{
                boxShadow: `0 0 0 2px var(--ink),0 0 0 3.5px ${avatarInfo(me.profile.avatar).bg}`,
              }}
            >
              <Avatar avatar={me.profile.avatar} size={36} alt="" />
            </button>
          )}
        </div>
        {/* fresh: no profile anywhere, so no game to host or join: no buttons */}
        <GameNight
          guest={me ? false : fresh ? null : unpaired || meError ? true : null}
        />
        {fresh && <SetupPending />}
        {unpaired && !fresh && (
          <div className="sub" style={{ marginTop: 14, fontSize: 13 }}>
            Hosting and solo training need a profile.{" "}
            <button
              type="button"
              className="link"
              style={{ padding: "12px 4px", margin: "-12px -4px" }}
              onClick={() => go("/pair")}
            >
              Got an invite code?
            </button>
          </div>
        )}
        {meError && <Problem error={meError} retry={reload} />}
        {me && (
          <Card style={{ marginTop: 10, padding: "12px 14px" }}>
            <div className="row between" style={{ gap: 8 }}>
              <div style={{ minWidth: 0 }}>
                <div className="kata" style={{ color: "var(--sky)" }}>
                  Solo training
                </div>
                <div style={{ fontWeight: 650, fontSize: 15, marginTop: 2 }}>
                  {deckLabel(today.deck, decks.data)}
                </div>
                <div className="sub" style={{ fontSize: 12.5 }}>
                  {today.length} songs · ~{Math.ceil((today.length * 25) / 60)}{" "}
                  min
                </div>
              </div>
              <div className="row" style={{ gap: 6, flex: "none" }}>
                <IconButton
                  icon="train"
                  label="Change deck"
                  onClick={() => go("/train")}
                />
                <Button
                  small
                  variant="sky"
                  icon="play"
                  disabled={training.busy}
                  onClick={() => training.start(today)}
                >
                  Start training
                </Button>
              </div>
            </div>
          </Card>
        )}
        {training.error && <Problem error={training.error} />}
        {loading && !me && !unpaired && !meError && (
          <div className="sub" style={{ marginTop: 18 }} role="status">
            Loading…
          </div>
        )}
      </div>
    </Screen>
  );
}
