// Test fakes for HTMLAudioElement and AudioContext: just what the player touches, with the calls recorded.
import { vi } from "vitest";
import type { AudioEnv } from "./player";

export class FakeParam {
  value = 0;
  calls: [string, ...number[]][] = [];
  cancelScheduledValues(t: number) {
    this.calls.push(["cancel", t]);
  }
  setValueAtTime(v: number, t: number) {
    this.value = v;
    this.calls.push(["set", v, t]);
  }
  linearRampToValueAtTime(v: number, t: number) {
    this.calls.push(["ramp", v, t]);
    this.value = v;
  }
  last(kind: string) {
    return [...this.calls].reverse().find((c) => c[0] === kind);
  }
}

// Listeners: just addEventListener/removeEventListener and a fire() for the test.
class Events {
  private ls = new Map<string, Set<() => void>>();
  addEventListener(type: string, f: () => void) {
    if (!this.ls.has(type)) this.ls.set(type, new Set());
    this.ls.get(type)!.add(f);
  }
  removeEventListener(type: string, f: () => void) {
    this.ls.get(type)?.delete(f);
  }
  fire(type: string) {
    [...(this.ls.get(type) ?? [])].forEach((f) => f());
  }
  listening(type: string) {
    return this.ls.get(type)?.size ?? 0;
  }
}

export class FakeContext extends Events {
  currentTime = 10;
  state = "suspended";
  destination = {};
  gain = { gain: new FakeParam(), connect: vi.fn() };
  sources: { el: FakeAudio; connect: ReturnType<typeof vi.fn> }[] = [];
  resume = vi.fn(async () => {
    this.state = "running";
  });
  close = vi.fn(async () => {});
  createGain() {
    return this.gain;
  }
  createMediaElementSource(el: FakeAudio) {
    const s = { el, connect: vi.fn() };
    this.sources.push(s);
    return s;
  }
}

export class FakeAudio extends Events {
  crossOrigin: string | null = null;
  preload = "";
  duration = NaN;
  // readyState 4 (all loaded) unless a test says otherwise; seekBlocked: seeks are ignored (no metadata yet)
  readyState = 4;
  seekBlocked = false;
  private _time = 0;
  get currentTime() {
    return this._time;
  }
  set currentTime(v: number) {
    if (!this.seekBlocked) this._time = v;
  }
  paused = true;
  ended = false;
  muted = false;
  // crossOrigin as it was when src was set: it must come first
  corsAtSrc: string | null = null;
  volumeWorks = true;
  rejectPlay = false;
  private _src = "";
  private _vol = 1;
  get src() {
    return this._src;
  }
  set src(v: string) {
    this.corsAtSrc = this.crossOrigin;
    this._src = v;
  }
  get volume() {
    return this._vol;
  }
  set volume(v: number) {
    if (this.volumeWorks) this._vol = v;
  }
  // deferPlay: play() stays pending until the test rejects it (a load cut short by a newer clip)
  deferPlay = false;
  rejecters: ((e: Error) => void)[] = [];
  play = vi.fn(() => {
    if (this.rejectPlay) return Promise.reject(new Error("NotAllowedError"));
    this.paused = false;
    if (this.deferPlay)
      return new Promise<void>((_, reject) => this.rejecters.push(reject));
    return Promise.resolve();
  });
  pause = vi.fn(() => {
    this.paused = true;
  });
  load = vi.fn();
  removeAttribute = vi.fn((a: string) => {
    if (a === "src") this._src = "";
  });
}

export function fakeEnv(
  opts: { webAudio?: boolean; volumeWorks?: boolean } = {},
) {
  const audios: FakeAudio[] = [];
  const ctxs: FakeContext[] = [];
  const nav = { audioSession: { type: "auto" } };
  const gestures = new EventTarget();
  const env: AudioEnv = {
    createAudio: () => {
      const a = new FakeAudio();
      a.volumeWorks = opts.volumeWorks ?? true;
      audios.push(a);
      return a as unknown as HTMLAudioElement;
    },
    createContext: () => {
      if (opts.webAudio === false) return null;
      const c = new FakeContext();
      ctxs.push(c);
      return c as unknown as AudioContext;
    },
    navigator: nav as unknown as Navigator,
    gestureTarget: gestures,
  };
  return { env, audios, ctxs, nav, gestures };
}
