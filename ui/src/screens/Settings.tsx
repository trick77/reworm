import { useState } from "react";
import { useLocation } from "wouter";
import { useApi, useAsync, useMe } from "../api/context";
import { player, saveEngine, savePrefetch, type Engine } from "../audio/player";
import { usePlayer } from "../audio/usePlayer";
import { Avatar } from "../components/Avatar";
import { Card, Segmented } from "../components/Controls";
import { Screen, Toggle } from "./common";

export function Settings() {
  const api = useApi();
  const [, go] = useLocation();
  const { me } = useMe();
  const devices = useAsync(me ? "devices" : null, () => api.devices());
  const version = useAsync("version", () => api.version());
  const clip = usePlayer();
  const [prefetch, setPrefetch] = useState(() => clip.player.prefetching);

  // a tap: the fresh elements unlock inside it
  const setEngine = (e: Engine) => {
    player().setEngine(e);
    player().unlock();
    saveEngine(e);
  };
  const togglePrefetch = (on: boolean) => {
    setPrefetch(on);
    player().setPrefetch(on);
    player().unlock();
    savePrefetch(on);
  };

  return (
    <Screen tab="settings">
      <div className="h-title" style={{ fontSize: 26 }}>
        Settings
      </div>
      {me && (
        <button
          type="button"
          className="card row"
          onClick={() => go("/me")}
          style={{ width: "100%", marginTop: 14, gap: 14, textAlign: "left" }}
        >
          <Avatar avatar={me.profile.avatar} size={52} alt="" />
          <div style={{ flex: 1 }}>
            <div style={{ fontWeight: 700, fontSize: 16 }}>
              {me.profile.name}
            </div>
            <div className="sub" style={{ fontSize: 12.5 }}>
              Profile, avatar &amp; devices
              {devices.data ? ` · ${devices.data.length} devices` : ""}
            </div>
          </div>
          <span className="sub">›</span>
        </button>
      )}
      <div className="cols">
        <div className="col">
          <div className="label">Audio</div>
          <Card style={{ padding: "4px 16px" }}>
            <div className="set-row">
              <div>
                <div>Audio engine</div>
                <div className="m">
                  Web Audio fades on iPhone too; Plain is the fallback if the
                  silent switch mutes it.
                </div>
              </div>
            </div>
            <div style={{ paddingBottom: 12 }}>
              <Segmented<Engine>
                label="Audio engine"
                value={clip.engine}
                onChange={setEngine}
                options={[
                  { value: "webaudio", label: "Web Audio" },
                  { value: "element", label: "Plain" },
                ]}
              />
            </div>
            <div className="set-row">
              <div>
                <div>Preload the next clip</div>
                <div className="m">
                  A second player loads the next song, so it starts without a
                  gap
                </div>
              </div>
              <Toggle
                on={prefetch}
                onChange={togglePrefetch}
                label="Preload the next clip"
              />
            </div>
            <div className="set-row">
              <div>
                <div>Fades</div>
                <div className="m">
                  1.5 s in, 0.8 s out · on answer and skip too
                </div>
              </div>
              <span className="sub" style={{ fontSize: 13 }}>
                always
              </span>
            </div>
          </Card>
          <div className="label">Release year</div>
          <Card className="sub" style={{ fontSize: 13, lineHeight: 1.5 }}>
            Always the <b className="paper">initial release</b> year. Never a
            remaster, re-release or compilation date. Card sets use the card's
            year; everything else uses the earliest release found for the song.
          </Card>
        </div>
        <div className="col">
          <div className="label">Keyboard</div>
          <Card className="sub" style={{ fontSize: 13, lineHeight: 1.9 }}>
            <span className="kbd">←</span> earlier &nbsp;{" "}
            <span className="kbd">space</span> spot on &nbsp;{" "}
            <span className="kbd">→</span> later &nbsp;{" "}
            <span className="kbd">⏎</span> next / lock in &nbsp;{" "}
            <span className="kbd">R</span> replay &nbsp;{" "}
            <span className="kbd">P</span> pause
          </Card>
          <div className="label">About</div>
          <Card className="sub" style={{ fontSize: 13 }}>
            Reworm{version.data ? ` ${version.data}` : ""} · self-hosted, Apple
            Music previews.
          </Card>
        </div>
      </div>
      <div style={{ height: 10 }} />
    </Screen>
  );
}
