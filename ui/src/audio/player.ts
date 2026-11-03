// Preview player: plays an Apple Music preview (90 s AAC, CORS *) with a fade in and out, as the mock's
// audio section (AUD, fadeIn, fadeOut, startCardAudio) does.
//
// Two engines, switchable at runtime, a hedge for milestone 0 (real iPhone/iPad):
// - "webaudio": <audio> → MediaElementSource → GainNode, fades by linearRampToValueAtTime. iOS Safari ignores
//   audio.volume, so this is the only way to fade there. Risk: Web Audio may follow the silent switch.
// - "element": the plain <audio>; fades by stepping volume where volume works, none where it doesn't (iOS).
// If iOS mutes Web Audio on the silent switch, the default flips to "element".
//
// One main element is reused for every clip (only its src changes): iOS lets an element that played once
// from a tap play again without one, which is what lets the host's rounds run unattended.

export type Engine = "webaudio" | "element";

export const FADE_IN = 1.5;
export const FADE_OUT = 0.8;
const PAUSE_FADE = 0.3;
const TICK_MS = 100;
const STEP_MS = 50;
const STOP_SLACK_MS = 30;

export type ClipState = {
  url: string | null;
  playing: boolean;
  pos: number; // seconds into the clip
  total: number; // clip length in seconds (Infinity until known)
  level: number; // 0–1, the fade
  engine: Engine;
};

export type PlayOptions = {
  from?: number; // seconds into the preview
  length?: number; // clip length in seconds; default: to the preview's end
};

export type AudioEnv = {
  createAudio: () => HTMLAudioElement;
  // null: no Web Audio here, the player falls back to the element engine
  createContext: () => AudioContext | null;
  navigator?: Navigator;
};

type AudioSessionNavigator = Navigator & { audioSession?: { type: string } };

// setAudioSession asks Safari 17+ to treat the page's audio as playback, which should keep it audible with
// the silent switch on (to verify in milestone 0). Elsewhere it does nothing.
export function setAudioSession(nav?: Navigator) {
  try {
    const s = (nav as AudioSessionNavigator | undefined)?.audioSession;
    if (s) s.type = "playback";
  } catch {
    // a browser that rejects the type keeps its default
  }
}

// silentWav is a few samples of silence, played on unlock so the main element has played once from a tap.
export function silentWav(): string {
  const n = 8;
  const b = new Uint8Array(44 + n);
  const dv = new DataView(b.buffer);
  const tag = (o: number, s: string) => {
    for (let i = 0; i < 4; i++) b[o + i] = s.charCodeAt(i);
  };
  tag(0, "RIFF");
  dv.setUint32(4, 36 + n, true);
  tag(8, "WAVE");
  tag(12, "fmt ");
  dv.setUint32(16, 16, true);
  dv.setUint16(20, 1, true); // PCM
  dv.setUint16(22, 1, true); // mono
  dv.setUint32(24, 8000, true);
  dv.setUint32(28, 8000, true);
  dv.setUint16(32, 1, true);
  dv.setUint16(34, 8, true);
  tag(36, "data");
  dv.setUint32(40, n, true);
  b.fill(128, 44);
  return "data:audio/wav;base64," + btoa(String.fromCharCode(...b));
}

function browserEnv(): AudioEnv {
  return {
    createAudio: () => new Audio(),
    createContext: () => {
      const w = window as Window & { webkitAudioContext?: typeof AudioContext };
      const Ctor = window.AudioContext ?? w.webkitAudioContext;
      return Ctor ? new Ctor() : null;
    },
    navigator: typeof navigator === "undefined" ? undefined : navigator,
  };
}

const PREFETCH_MAX = 2;

export class PreviewPlayer {
  private env: AudioEnv;
  private _engine: Engine;
  private ctx: AudioContext | null = null;
  private gain: GainNode | null = null;
  private el: HTMLAudioElement | null = null;
  private routed = new WeakSet<HTMLAudioElement>();
  private url: string | null = null;
  private pre = new Map<string, HTMLAudioElement>();
  private from = 0;
  private length = Infinity;
  private playing = false;
  private fading = false;
  private ending = false;
  private volumeWorks: boolean | null = null;
  private stopT: ReturnType<typeof setTimeout> | undefined;
  private stepT: ReturnType<typeof setInterval> | undefined;
  private tickT: ReturnType<typeof setInterval> | undefined;
  private progressL = new Set<(s: ClipState) => void>();
  private endL = new Set<() => void>();

  constructor(opts: { engine?: Engine; env?: Partial<AudioEnv> } = {}) {
    this.env = { ...browserEnv(), ...opts.env };
    this._engine = opts.engine ?? "webaudio";
  }

  get engine(): Engine {
    return this._engine;
  }

  // setEngine switches engines. An element routed into Web Audio stays routed for good, so the switch
  // starts over with fresh elements.
  setEngine(e: Engine) {
    if (e === this._engine) return;
    this.stopNow();
    this.el = null;
    this.url = null;
    this.dropPrefetched();
    this._engine = e;
    this.emit();
  }

  // unlock runs on the first user gesture: audio session, the AudioContext, one silent play of the main
  // element.
  unlock() {
    setAudioSession(this.env.navigator);
    if (this._engine === "webaudio") this.ensureCtx();
    const el = this.main();
    if (this.url == null && el.paused) {
      el.src = silentWav();
      el.play()?.then(
        () => {
          if (this.url == null) el.pause();
        },
        () => {},
      );
    }
  }

  // prefetch starts loading the next clip so it starts without a gap.
  prefetch(url: string) {
    if (url === this.url || this.pre.has(url)) return;
    const el = this.newElement();
    el.src = url;
    el.load?.();
    this.pre.set(url, el);
    while (this.pre.size > PREFETCH_MAX) {
      const [oldest, old] = this.pre.entries().next().value!;
      old.removeAttribute("src");
      this.pre.delete(oldest);
    }
  }

  // play starts a clip at `from` with a fade in; it fades out into the clip's end and then fires onEnd.
  play(url: string, opts: PlayOptions = {}) {
    this.clearTimers();
    this.fading = false;
    this.ending = false;
    this.from = opts.from ?? 0;
    this.length = opts.length ?? Infinity;
    if (this._engine === "webaudio") this.ensureCtx();
    const el = this.main();
    if (this.url !== url) {
      el.src = url;
      this.url = url;
      const warm = this.pre.get(url);
      if (warm) {
        warm.removeAttribute("src");
        this.pre.delete(url);
      }
    }
    try {
      el.currentTime = this.from;
    } catch {
      // before metadata some browsers refuse; the clip then starts at 0
    }
    this.start(el);
  }

  // fadeOut fades the current clip out over `seconds`, then pauses. Resolves once paused.
  fadeOut(seconds = FADE_OUT): Promise<void> {
    const el = this.el;
    if (!el || el.paused || this.fading) return Promise.resolve();
    this.fading = true;
    if (this._engine === "webaudio" && this.gain && this.ctx) {
      const g = this.gain.gain;
      const now = this.ctx.currentTime;
      g.cancelScheduledValues(now);
      g.setValueAtTime(g.value, now);
      g.linearRampToValueAtTime(0.0001, now + seconds);
    } else {
      this.stepVolume(0, seconds);
    }
    clearTimeout(this.stopT);
    return new Promise((resolve) => {
      this.stopT = setTimeout(
        () => {
          clearInterval(this.stepT);
          el.pause();
          this.fading = false;
          this.playing = false;
          this.stopTick();
          const ended = this.ending;
          this.ending = false;
          this.emit();
          if (ended) this.endL.forEach((f) => f());
          resolve();
        },
        seconds * 1000 + STOP_SLACK_MS,
      );
    });
  }

  pause(): Promise<void> {
    return this.fadeOut(PAUSE_FADE);
  }

  resume() {
    const el = this.el;
    if (!el || !this.url || this.playing) return;
    this.clearTimers();
    this.fading = false;
    this.start(el);
  }

  // level is the current fade, 0–1.
  level(): number {
    if (!this.el || !this.playing) return 0;
    if (this._engine === "webaudio" && this.gain) return this.gain.gain.value;
    return this.el.volume;
  }

  state(): ClipState {
    const el = this.el;
    const dur =
      el && Number.isFinite(el.duration) ? el.duration - this.from : Infinity;
    const total = Math.min(this.length, dur);
    const pos = el && this.url ? Math.max(0, el.currentTime - this.from) : 0;
    return {
      url: this.url,
      playing: this.playing,
      pos: Math.min(pos, total),
      total,
      level: this.level(),
      engine: this._engine,
    };
  }

  // onProgress fires every 100 ms while playing and on every play, pause and end. Returns the unsubscribe.
  onProgress(f: (s: ClipState) => void): () => void {
    this.progressL.add(f);
    return () => this.progressL.delete(f);
  }

  onEnd(f: () => void): () => void {
    this.endL.add(f);
    return () => this.endL.delete(f);
  }

  destroy() {
    this.stopNow();
    this.dropPrefetched();
    this.progressL.clear();
    this.endL.clear();
    void this.ctx?.close?.();
    this.ctx = null;
    this.gain = null;
    this.el = null;
  }

  private start(el: HTMLAudioElement) {
    this.fadeIn(el);
    this.playing = true;
    el.play()?.catch(() => {
      // autoplay refused (no gesture yet): stays paused, the UI offers play
      this.playing = false;
      this.stopTick();
      this.emit();
    });
    this.startTick();
    this.emit();
  }

  private fadeIn(el: HTMLAudioElement) {
    if (this._engine === "webaudio" && this.gain && this.ctx) {
      this.route(el);
      const g = this.gain.gain;
      const now = this.ctx.currentTime;
      g.cancelScheduledValues(now);
      g.setValueAtTime(0.0001, now);
      g.linearRampToValueAtTime(1, now + FADE_IN);
      return;
    }
    if (this.canFadeVolume(el)) {
      el.volume = 0;
      this.stepVolume(1, FADE_IN);
    }
  }

  // stepVolume moves el.volume to `to` in 50 ms steps (element engine); without a working volume (iOS) it
  // does nothing and the clip simply stops.
  private stepVolume(to: number, seconds: number) {
    const el = this.el;
    clearInterval(this.stepT);
    if (!el || !this.canFadeVolume(el)) return;
    const steps = Math.max(1, Math.round((seconds * 1000) / STEP_MS));
    const delta = (to - el.volume) / steps;
    let n = 0;
    this.stepT = setInterval(() => {
      n++;
      el.volume = n >= steps ? to : Math.min(1, Math.max(0, el.volume + delta));
      if (n >= steps) clearInterval(this.stepT);
    }, STEP_MS);
  }

  private canFadeVolume(el: HTMLAudioElement): boolean {
    if (this.volumeWorks == null) {
      const v = el.volume;
      el.volume = 0.5;
      this.volumeWorks = Math.abs(el.volume - 0.5) < 0.01;
      el.volume = v;
    }
    return this.volumeWorks;
  }

  private tick() {
    const el = this.el;
    if (!el || !this.playing) return;
    const s = this.state();
    this.emit(s);
    if (this.fading) return;
    if (el.ended) {
      this.playing = false;
      this.stopTick();
      this.emit();
      this.endL.forEach((f) => f());
      return;
    }
    if (Number.isFinite(s.total) && s.pos >= s.total - FADE_OUT) {
      this.ending = true;
      void this.fadeOut(Math.max(0.05, Math.min(FADE_OUT, s.total - s.pos)));
    }
  }

  private emit(s: ClipState = this.state()) {
    this.progressL.forEach((f) => f(s));
  }

  private ensureCtx() {
    if (!this.ctx) {
      const ctx = this.env.createContext();
      if (!ctx) {
        this._engine = "element";
        return;
      }
      this.ctx = ctx;
      this.gain = ctx.createGain();
      this.gain.gain.value = 0;
      this.gain.connect(ctx.destination);
    }
    if (this.ctx.state === "suspended") void this.ctx.resume();
  }

  // route wires an element into the gain node, once: a MediaElementSource can't be made twice.
  private route(el: HTMLAudioElement) {
    if (!this.ctx || !this.gain || this.routed.has(el)) return;
    this.ctx.createMediaElementSource(el).connect(this.gain);
    this.routed.add(el);
  }

  private main(): HTMLAudioElement {
    if (!this.el) this.el = this.newElement();
    return this.el;
  }

  // crossOrigin before src, or Web Audio gets a tainted (silent) stream.
  private newElement(): HTMLAudioElement {
    const el = this.env.createAudio();
    el.crossOrigin = "anonymous";
    el.preload = "auto";
    return el;
  }

  private startTick() {
    this.stopTick();
    this.tickT = setInterval(() => this.tick(), TICK_MS);
  }

  private stopTick() {
    clearInterval(this.tickT);
    this.tickT = undefined;
  }

  private clearTimers() {
    clearTimeout(this.stopT);
    clearInterval(this.stepT);
    this.stopTick();
  }

  private stopNow() {
    this.clearTimers();
    this.el?.pause();
    this.playing = false;
    this.fading = false;
    this.ending = false;
  }

  private dropPrefetched() {
    this.pre.forEach((el) => el.removeAttribute("src"));
    this.pre.clear();
  }
}

const ENGINE_KEY = "reworm.audioEngine";

// defaultEngine: ?audio=element|webaudio, else the saved choice, else webaudio.
export function defaultEngine(): Engine {
  try {
    const q = new URLSearchParams(window.location.search).get("audio");
    if (q === "element" || q === "webaudio") return q;
    const saved = window.localStorage.getItem(ENGINE_KEY);
    if (saved === "element" || saved === "webaudio") return saved;
  } catch {
    // storage blocked (private mode): the default
  }
  return "webaudio";
}

export function saveEngine(e: Engine) {
  try {
    window.localStorage.setItem(ENGINE_KEY, e);
  } catch {
    // not saved; the default applies next time
  }
}

let shared: PreviewPlayer | null = null;

// player is the app's one preview player.
export function player(): PreviewPlayer {
  shared ??= new PreviewPlayer({ engine: defaultEngine() });
  return shared;
}
