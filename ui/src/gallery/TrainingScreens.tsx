// The mock's training play and reveal screens, composed from the components with demo data, to show the
// .cols.mid.wide layout (two columns on iPad landscape, stacked on iPad portrait). The training PR builds
// the real ones.
import { AppShell } from "../components/AppShell";
import { Button, IconButton } from "../components/Controls";
import { Cover, SongHead } from "../components/Cover";
import { Icon } from "../components/Icon";
import { Verdict } from "../components/Verdict";
import { Vinyl } from "../components/Vinyl";
import { ClipTime, Waveform } from "../components/Waveform";
import { YearCard } from "../components/YearCard";
import { SONGS } from "./demo";

export function TrainPlay() {
  return (
    <AppShell scrollClassName="playscr">
      <div className="row" style={{ padding: "2px 0 8px" }}>
        <IconButton icon="x" label="End session" />
      </div>
      <div className="cols mid wide">
        <div className="col">
          <Vinyl spinning />
          <Waveform progress={0.4} />
          <ClipTime pos={8} total={20} level={1} note="♪ live preview" />
          <div className="player-ctl">
            <button type="button">
              <Icon name="pause" />
              Pause
            </button>
            <button type="button">
              <Icon name="replay" />
              Replay
            </button>
          </div>
        </div>
        <div className="col">
          <YearCard year={1987} onAnswer={() => {}} choices />
        </div>
      </div>
    </AppShell>
  );
}

export function TrainReveal() {
  const s = SONGS[0];
  return (
    <AppShell
      footer={
        <Button>
          Next song <span className="kbd">⏎</span>
        </Button>
      }
    >
      <div className="row" style={{ padding: "2px 0 8px" }}>
        <IconButton icon="x" label="End session" />
      </div>
      <div className="cols wide">
        <div className="col">
          <SongHead
            cover={
              <Cover
                title={s.title}
                artist={s.artist}
                album={s.album}
                src={s.art}
                flip
              />
            }
            title={s.title}
            artist={s.artist}
            meta={`Pop · ${s.album}`}
          />
          <Verdict
            result="hit"
            year={s.year}
            what="Yes, later — 2 years after 1987"
            tier={{ label: "Tight one · 2 yrs off", color: "var(--gold)" }}
            why="+20 XP · streak 3"
            ruler={{ shown: 1987 }}
          />
        </div>
        <div className="col" />
      </div>
    </AppShell>
  );
}
