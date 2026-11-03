// Preview player: plays an Apple Music preview (90 s AAC, CORS *) with a fade in and out, as the mock's
// audio section (AUD, fadeIn, fadeOut, startCardAudio) does.
//
// Two engines, switchable at runtime, a hedge for milestone 0 (real iPhone/iPad):
// - "webaudio": <audio> → MediaElementSource → GainNode, fades by linearRampToValueAtTime. iOS Safari ignores
//   audio.volume, so this is the only way to fade there. Risk: Web Audio may follow the silent switch.
// - "element": the plain <audio>; fades by stepping volume where volume works, none where it doesn't (iOS).
// If iOS mutes Web Audio on the silent switch, the default flips to "element".
//
// Two elements, A/B: the active one plays, the idle one preloads the next clip, and play() of that clip swaps
// them. Both are created and played once (muted) inside the first tap: iOS lets an element that played from a
// tap play again without one, which is what lets the host's rounds run unattended. If iOS refuses the second
// element, `prefetch: false` (?prefetch=off) is the fallback: one element, no preloading.

export type Engine = "webaudio" | "element";

export const FADE_IN = 1.5;
export const FADE_OUT = 0.8;
const PAUSE_FADE = 0.3;
const TICK_MS = 100;
const STEP_MS = 50;
const STOP_SLACK_MS = 30;
const SEEK_SLACK = 0.25;

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
  // where user gestures arrive (window): a context iOS suspended or interrupted resumes on the next one
  gestureTarget?: EventTarget;
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

// silentWav is a few samples of silence: what an element plays on unlock when it holds no clip yet.
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
    gestureTarget: typeof window === "undefined" ? undefined : window,
  };
}

const GESTURES = ["pointerdown", "keydown", "touchend"] as const;

export class PreviewPlayer {
  private env: AudioEnv;
  private _engine: Engine;
  private readonly buffers: number;
  private ctx: AudioContext | null = null;
  private gain: GainNode | null = null;
  // A/B: els[active] plays, the other preloads; srcs[i] is the clip URL element i holds (null: none/silence)
  private els: HTMLAudioElement[] = [];
  private srcs: (string | null)[] = [];
  private active = 0;
  private routed = new WeakSet<HTMLAudioElement>();
  private primed = new WeakSet<HTMLAudioElement>();
  private from = 0;
  private length = Infinity;
  private playing = false;
  private fading = false;
  private ending = false;
  private ended = false;
  // bumped on every start: a play() promise of an older start may reject (AbortError) and must not stop this one
  private gen = 0;
  private fadeP: Promise<void> | null = null;
  private settle: (() => void) | null = null;
  private volumeWorks: boolean | null = null;
  private stopT: ReturnType<typeof setTimeout> | undefined;
  private stepT: ReturnType<typeof setInterval> | undefined;
  private tickT: ReturnType<typeof setInterval> | undefined;
  private progressL = new Set<(s: ClipState) => void>();
  private endL = new Set<() => void>();

  constructor(
    opts: { engine?: Engine; prefetch?: boolean; env?: Partial<AudioEnv> } = {},
  ) {
    this.env = { ...browserEnv(), ...opts.env };
    this._engine = opts.engine ?? "webaudio";
    this.buffers = opts.prefetch === false ? 1 : 2;
  }

  get engine(): Engine {
    return this._engine;
  }

  // setEngine switches engines. An element routed into Web Audio stays routed for good, so the switch starts
  // over with fresh elements; call unlock() from the same tap.
  setEngine(e: Engine) {
    if (e === this._engine) return;
    this.stopNow();
    this.els.forEach(release);
    this.els = [];
    this.srcs = [];
    this.active = 0;
    this._engine = e;
    this.emit();
  }

  // unlock runs inside a user gesture: audio session, the AudioContext, and one muted play of every element,
  // so each may play later without a tap. Each element pauses once its play() resolved (pausing earlier
  // aborts the play, which may not count as played), unless a clip started on it meanwhile.
  unlock() {
    setAudioSession(this.env.navigator);
    if (this._engine === "webaudio") this.ensureCtx();
    for (let i = 0; i < this.buffers; i++) {
      const el = this.element(i);
      // once per element: a later unlock (every tap may call it) must not start a preloaded element
      if (this.primed.has(el) || !el.paused) continue;
      this.primed.add(el);
      if (this.srcs[i] == null) el.src = silentWav();
      el.muted = true;
      const gen = this.gen;
      const unmute = () => {
        el.muted = false;
      };
      const p = el.play();
      if (!p) {
        el.pause();
        unmute();
        continue;
      }
      p.then(() => {
        if (this.gen === gen || el !== this.current()) el.pause();
        unmute();
      }, unmute);
    }
  }

  // prefetch loads the next clip into the idle element, so play() of it starts without a gap.
  prefetch(url: string) {
    if (this.buffers < 2 || url === this.srcs[this.active]) return;
    const idle = 1 - this.active;
    if (this.srcs[idle] === url) return;
    const el = this.element(idle);
    el.src = url;
    el.load?.();
    this.srcs[idle] = url;
  }

  // play starts a clip at `from` with a fade in; it fades out into the clip's end and then fires onEnd, once.
  play(url: string, opts: PlayOptions = {}) {
    this.clearTimers();
    this.fading = false;
    this.ending = false;
    this.ended = false;
    this.from = opts.from ?? 0;
    this.length = opts.length ?? Infinity;
    if (this._engine === "webaudio") this.ensureCtx();
    const idle = 1 - this.active;
    if (
      this.buffers > 1 &&
      this.srcs[idle] === url &&
      this.srcs[this.active] !== url
    ) {
      this.els[this.active]?.pause();
      this.active = idle;
    } else if (this.srcs[this.active] !== url) {
      this.element(this.active).src = url;
      this.srcs[this.active] = url;
    }
    const el = this.element(this.active);
    const at = this.from;
    const seek = () => {
      try {
        el.currentTime = at;
      } catch {
        // before metadata some browsers refuse: loadedmetadata below tries again
      }
    };
    seek();
    this.start(el);
    // A src just set has no metadata yet: the seek may not stick and the clip would start at 0 and run past
    // its end. Seek again once the metadata is in, if this start is still the current one.
    if (el.readyState >= 1 && Math.abs(el.currentTime - at) < SEEK_SLACK)
      return;
    const gen = this.gen;
    const onMeta = () => {
      el.removeEventListener("loadedmetadata", onMeta);
      if (gen !== this.gen || Math.abs(el.currentTime - at) < SEEK_SLACK)
        return;
      seek();
      this.emit();
    };
    el.addEventListener("loadedmetadata", onMeta);
  }

  // fadeOut fades the current clip out over `seconds`, then pauses. Resolves once paused, or when a new
  // play/resume cuts the fade short.
  fadeOut(seconds = FADE_OUT): Promise<void> {
    const el = this.current();
    if (this.fadeP) return this.fadeP;
    if (!el || el.paused || !this.playing) return Promise.resolve();
    this.fading = true;
    if (this._engine === "webaudio" && this.gain && this.ctx) {
      const g = this.gain.gain;
      const now = this.ctx.currentTime;
      g.cancelScheduledValues(now);
      g.setValueAtTime(g.value, now);
      g.linearRampToValueAtTime(0.0001, now + seconds);
    } else {
      this.stepVolume(el, 0, seconds);
    }
    this.fadeP = new Promise((resolve) => {
      this.settle = resolve;
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
          this.settleFade();
          if (ended) this.end();
        },
        seconds * 1000 + STOP_SLACK_MS,
      );
    });
    return this.fadeP;
  }

  pause(): Promise<void> {
    return this.fadeOut(PAUSE_FADE);
  }

  // resume continues a paused clip, takes back a pause fade, or replays a clip that ended or is fading out
  // into its end from its start (tick would only fade it out again).
  resume() {
    const el = this.current();
    const url = this.srcs[this.active];
    if (!el || !url) return;
    if (this.ended || this.ending) {
      this.play(url, { from: this.from, length: this.length });
      return;
    }
    const takeBack = this.fading;
    if (takeBack) {
      this.clearTimers();
      this.fading = false;
      this.ending = false;
    } else if (this.playing) {
      return;
    }
    this.start(el, !takeBack);
  }

  // level is the current fade, 0–1.
  level(): number {
    const el = this.current();
    if (!el || !this.playing) return 0;
    if (this._engine === "webaudio" && this.gain) return this.gain.gain.value;
    return el.volume;
  }

  state(): ClipState {
    const el = this.current();
    const url = this.srcs[this.active] ?? null;
    const dur =
      el && Number.isFinite(el.duration) ? el.duration - this.from : Infinity;
    const total = Math.min(this.length, dur);
    const pos = el && url ? Math.max(0, el.currentTime - this.from) : 0;
    return {
      url,
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

  // start plays el with a fade in, from silence unless it takes back a fade out still sounding.
  private start(el: HTMLAudioElement, fromSilence = true) {
    const gen = ++this.gen;
    el.muted = false; // an unlock's muted play may still be pending on this element
    this.fadeIn(el, fromSilence);
    this.playing = true;
    el.play()?.catch(() => {
      // autoplay refused (no tap yet) or cut short by a newer start: only the newest start counts
      if (gen !== this.gen) return;
      this.playing = false;
      this.stopTick();
      this.emit();
    });
    this.startTick();
    this.emit();
  }

  // fadeIn ramps up to full, from silence or from where a taken-back fade out is.
  private fadeIn(el: HTMLAudioElement, fromSilence: boolean) {
    if (this._engine === "webaudio" && this.gain && this.ctx) {
      this.route(el);
      const g = this.gain.gain;
      const now = this.ctx.currentTime;
      const from = fromSilence ? 0.0001 : Math.max(0.0001, g.value);
      g.cancelScheduledValues(now);
      g.setValueAtTime(from, now);
      g.linearRampToValueAtTime(1, now + FADE_IN);
      return;
    }
    if (this.canFadeVolume(el)) {
      if (fromSilence) el.volume = 0;
      this.stepVolume(el, 1, FADE_IN);
    }
  }

  // stepVolume moves el.volume to `to` in 50 ms steps (element engine); without a working volume (iOS) it
  // does nothing and the clip simply stops.
  private stepVolume(el: HTMLAudioElement, to: number, seconds: number) {
    clearInterval(this.stepT);
    if (!this.canFadeVolume(el)) return;
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
    const el = this.current();
    if (!el || !this.playing) return;
    const s = this.state();
    this.emit(s);
    if (this.fading) return;
    if (el.ended) {
      this.playing = false;
      this.stopTick();
      this.emit();
      this.end();
      return;
    }
    if (Number.isFinite(s.total) && s.pos >= s.total - FADE_OUT) {
      this.ending = true;
      void this.fadeOut(Math.max(0.05, Math.min(FADE_OUT, s.total - s.pos)));
    }
  }

  // end fires onEnd once per play.
  private end() {
    if (this.ended) return;
    this.ended = true;
    this.endL.forEach((f) => f());
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
      // iOS suspends or "interrupts" the context (a call, Siri) and wants a gesture to resume it
      ctx.addEventListener?.("statechange", () => {
        if (stalled(ctx)) this.resumeOnGesture(ctx);
      });
    }
    resumeCtx(this.ctx);
  }

  private gestureArmed = false;

  // resumeOnGesture resumes a stalled context on the next tap or key anywhere, once.
  private resumeOnGesture(ctx: AudioContext) {
    const t = this.env.gestureTarget;
    if (!t || this.gestureArmed) return;
    this.gestureArmed = true;
    const on = () => {
      GESTURES.forEach((g) => t.removeEventListener(g, on, true));
      this.gestureArmed = false;
      resumeCtx(ctx);
    };
    GESTURES.forEach((g) => t.addEventListener(g, on, true));
  }

  // route wires an element into the shared gain node, once: a MediaElementSource can't be made twice.
  private route(el: HTMLAudioElement) {
    if (!this.ctx || !this.gain || this.routed.has(el)) return;
    this.ctx.createMediaElementSource(el).connect(this.gain);
    this.routed.add(el);
  }

  private current(): HTMLAudioElement | undefined {
    return this.els[this.active];
  }

  // element i, created on first use. crossOrigin before any src, or Web Audio gets a tainted (silent) stream.
  private element(i: number): HTMLAudioElement {
    if (!this.els[i]) {
      const el = this.env.createAudio();
      el.crossOrigin = "anonymous";
      el.preload = "auto";
      this.els[i] = el;
      this.srcs[i] = null;
    }
    return this.els[i];
  }

  private startTick() {
    this.stopTick();
    this.tickT = setInterval(() => this.tick(), TICK_MS);
  }

  private stopTick() {
    clearInterval(this.tickT);
    this.tickT = undefined;
  }

  private settleFade() {
    const s = this.settle;
    this.settle = null;
    this.fadeP = null;
    s?.();
  }

  // clearTimers also settles a fade promise whose timer it cancels: nobody waits forever.
  private clearTimers() {
    clearTimeout(this.stopT);
    clearInterval(this.stepT);
    this.stopTick();
    this.settleFade();
  }

  private stopNow() {
    this.clearTimers();
    this.current()?.pause();
    this.playing = false;
    this.fading = false;
    this.ending = false;
  }
}

// stalled: neither running nor closed: "suspended", or iOS's "interrupted".
function stalled(ctx: AudioContext): boolean {
  const s: string = ctx.state;
  return s !== "running" && s !== "closed";
}

function resumeCtx(ctx: AudioContext) {
  if (stalled(ctx)) ctx.resume().catch(() => {});
}

// release stops an element's download too: removing src alone keeps the old resource loading until load().
function release(el: HTMLAudioElement) {
  el.pause();
  el.removeAttribute("src");
  el.load?.();
}

const ENGINE_KEY = "reworm.audioEngine";

function query(key: string): string | null {
  return new URLSearchParams(window.location.search).get(key);
}

// defaultEngine: ?audio=element|webaudio, else the saved choice, else webaudio.
export function defaultEngine(): Engine {
  try {
    const q = query("audio");
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

// defaultPrefetch: on, unless ?prefetch=off (the single-element fallback, should iOS refuse a second element).
export function defaultPrefetch(): boolean {
  return query("prefetch") !== "off";
}

let shared: PreviewPlayer | null = null;

// player is the app's one preview player.
export function player(): PreviewPlayer {
  shared ??= new PreviewPlayer({
    engine: defaultEngine(),
    prefetch: defaultPrefetch(),
  });
  return shared;
}
