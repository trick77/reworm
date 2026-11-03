import { fakeEnv } from "./fakes";
import {
  FADE_IN,
  FADE_OUT,
  PreviewPlayer,
  defaultEngine,
  defaultPrefetch,
  player,
  saveEngine,
  setAudioSession,
  silentWav,
} from "./player";

const A = "https://audio.example/a.m4a";
const B = "https://audio.example/b.m4a";
const C = "https://audio.example/c.m4a";

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("PreviewPlayer, webaudio engine", () => {
  function setup() {
    const f = fakeEnv();
    const p = new PreviewPlayer({ engine: "webaudio", env: f.env });
    return { ...f, p };
  }

  it("plays through the gain node with a 1.5 s fade in from `from`", () => {
    const { p, audios, ctxs } = setup();
    p.play(A, { from: 12, length: 20 });
    const el = audios[0];
    const ctx = ctxs[0];
    expect(el.src).toBe(A);
    expect(el.corsAtSrc).toBe("anonymous");
    expect(el.currentTime).toBe(12);
    expect(el.play).toHaveBeenCalled();
    expect(ctx.sources).toHaveLength(1);
    expect(ctx.sources[0].connect).toHaveBeenCalledWith(ctx.gain);
    expect(ctx.resume).toHaveBeenCalled();
    const g = ctx.gain.gain;
    expect(g.last("set")).toEqual(["set", 0.0001, 10]);
    expect(g.last("ramp")).toEqual(["ramp", 1, 10 + FADE_IN]);
    expect(p.state()).toMatchObject({ url: A, playing: true, total: 20 });
  });

  it("fades out over 0.8 s, then pauses", async () => {
    const { p, audios, ctxs } = setup();
    p.play(A);
    const done = vi.fn();
    void p.fadeOut().then(done);
    expect(ctxs[0].gain.gain.last("ramp")).toEqual([
      "ramp",
      0.0001,
      10 + FADE_OUT,
    ]);
    await vi.advanceTimersByTimeAsync(800);
    expect(audios[0].paused).toBe(false);
    await vi.advanceTimersByTimeAsync(40);
    expect(audios[0].paused).toBe(true);
    expect(done).toHaveBeenCalled();
    expect(p.state().playing).toBe(false);
    expect(p.level()).toBe(0);
  });

  it("ignores a second fadeOut while fading and one with nothing playing", async () => {
    const { p, ctxs } = setup();
    await p.fadeOut();
    p.play(A);
    void p.fadeOut();
    const ramps = ctxs[0].gain.gain.calls.filter((c) => c[0] === "ramp");
    void p.fadeOut(2);
    expect(ctxs[0].gain.gain.calls.filter((c) => c[0] === "ramp")).toEqual(
      ramps,
    );
  });

  it("fades out into the clip's end, then fires onEnd", async () => {
    const { p, audios } = setup();
    const end = vi.fn();
    const seen: number[] = [];
    p.onEnd(end);
    p.onProgress((s) => seen.push(s.pos));
    p.play(A, { from: 30, length: 20 });
    const el = audios[0];
    el.currentTime = 35;
    await vi.advanceTimersByTimeAsync(100);
    expect(seen.at(-1)).toBe(5);
    el.currentTime = 49.4; // 19.4 s in: inside the last 0.8 s
    await vi.advanceTimersByTimeAsync(100);
    expect(el.paused).toBe(false);
    expect(end).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(700);
    expect(el.paused).toBe(true);
    expect(end).toHaveBeenCalledTimes(1);
  });

  it("fires onEnd when the preview itself runs out", async () => {
    const { p, audios } = setup();
    const end = vi.fn();
    const off = p.onEnd(end);
    p.play(A);
    audios[0].ended = true;
    await vi.advanceTimersByTimeAsync(100);
    expect(end).toHaveBeenCalledTimes(1);
    expect(p.state().playing).toBe(false);
    off();
  });

  it("caps the clip at the preview's duration", () => {
    const { p, audios } = setup();
    p.play(A, { from: 80, length: 20 });
    audios[0].duration = 90;
    expect(p.state().total).toBe(10);
  });

  it("reuses one element for every clip and routes it once", () => {
    const { p, audios, ctxs } = setup();
    p.play(A);
    p.play(B);
    p.play(B, { from: 5 });
    expect(audios).toHaveLength(1);
    expect(audios[0].src).toBe(B);
    expect(audios[0].currentTime).toBe(5);
    expect(ctxs[0].sources).toHaveLength(1);
  });

  it("pauses with a short fade and resumes with a fade in", async () => {
    const { p, audios, ctxs } = setup();
    p.play(A);
    void p.pause();
    expect(ctxs[0].gain.gain.last("ramp")).toEqual(["ramp", 0.0001, 10.3]);
    await vi.advanceTimersByTimeAsync(400);
    expect(audios[0].paused).toBe(true);
    ctxs[0].currentTime = 20;
    p.resume();
    expect(audios[0].play).toHaveBeenCalledTimes(2);
    expect(ctxs[0].gain.gain.last("ramp")).toEqual(["ramp", 1, 20 + FADE_IN]);
    expect(p.state().playing).toBe(true);
    p.resume(); // already playing: nothing
    expect(audios[0].play).toHaveBeenCalledTimes(2);
  });

  it("does nothing on resume before any clip", () => {
    const { p, audios } = setup();
    p.resume();
    expect(audios).toHaveLength(0);
  });

  it("level follows the gain", () => {
    const { p, ctxs } = setup();
    expect(p.level()).toBe(0);
    p.play(A);
    ctxs[0].gain.gain.value = 0.42;
    expect(p.level()).toBe(0.42);
  });

  it("marks the clip stopped when play is refused", async () => {
    const f = fakeEnv();
    const p = new PreviewPlayer({
      env: {
        ...f.env,
        createAudio: () => {
          const el = f.env.createAudio();
          (el as unknown as { rejectPlay: boolean }).rejectPlay = true;
          return el;
        },
      },
    });
    p.play(A);
    await vi.advanceTimersByTimeAsync(0);
    expect(p.state().playing).toBe(false);
  });

  it("unlock: audio session, context, one muted play+pause of both elements in the tap", () => {
    const { p, audios, ctxs, nav } = setup();
    p.unlock();
    expect(nav.audioSession.type).toBe("playback");
    expect(ctxs[0].resume).toHaveBeenCalled();
    expect(audios).toHaveLength(2);
    for (const el of audios) {
      expect(el.src).toBe(silentWav());
      expect(el.play).toHaveBeenCalledTimes(1);
      expect(el.pause).toHaveBeenCalledTimes(1);
      expect(el.paused).toBe(true);
      expect(el.muted).toBe(false);
    }
    expect(p.state().url).toBeNull();
  });

  it("unlock leaves a playing element and a preloaded clip alone", () => {
    const { p, audios } = setup();
    p.play(A);
    p.prefetch(B);
    p.unlock();
    expect(audios[0].play).toHaveBeenCalledTimes(1);
    expect(audios[0].pause).not.toHaveBeenCalled();
    expect(audios[1].src).toBe(B);
    expect(audios[1].play).toHaveBeenCalledTimes(1);
  });

  it("A/B: the idle element preloads, play swaps to it, each routed once", () => {
    const { p, audios, ctxs } = setup();
    p.play(A);
    p.prefetch(B);
    p.prefetch(B);
    p.prefetch(A); // already the active clip
    expect(audios).toHaveLength(2);
    expect(audios[1]).toMatchObject({ src: B, preload: "auto" });
    expect(audios[1].corsAtSrc).toBe("anonymous");
    expect(audios[1].load).toHaveBeenCalledTimes(1);
    p.play(B);
    expect(audios[0].paused).toBe(true);
    expect(audios[1].paused).toBe(false);
    expect(p.state().url).toBe(B);
    p.prefetch(C); // into A's element, now idle
    expect(audios[0].src).toBe(C);
    p.play(C);
    expect(audios[0].paused).toBe(false);
    expect(audios[1].paused).toBe(true);
    expect(audios).toHaveLength(2);
    expect(ctxs[0].sources.map((s) => s.el)).toEqual([audios[0], audios[1]]);
  });

  it("plays a clip that was not preloaded on the active element", () => {
    const { p, audios } = setup();
    p.play(A);
    p.prefetch(B);
    p.play(C);
    expect(audios[0].src).toBe(C);
    expect(audios[1].src).toBe(B);
  });

  it("prefetch: false is one element, no preloading", () => {
    const f = fakeEnv();
    const p = new PreviewPlayer({ prefetch: false, env: f.env });
    p.unlock();
    p.play(A);
    p.prefetch(B);
    p.play(B);
    expect(f.audios).toHaveLength(1);
    expect(f.audios[0].src).toBe(B);
  });

  it("a superseded start's rejected play() does not stop the new clip", async () => {
    const { p, audios } = setup();
    p.play(A);
    const el = audios[0];
    el.deferPlay = true;
    p.play(B); // skip mid-load
    p.play(C);
    el.rejecters[0](new DOMException("interrupted", "AbortError"));
    await vi.advanceTimersByTimeAsync(0);
    expect(p.state()).toMatchObject({ url: C, playing: true });
    el.rejecters[1](new DOMException("refused", "NotAllowedError"));
    await vi.advanceTimersByTimeAsync(0);
    expect(p.state().playing).toBe(false);
  });

  it("settles a fade's promise when a new clip cuts it short", async () => {
    const { p } = setup();
    p.play(A);
    const done = vi.fn();
    void p.pause().then(done);
    p.play(B);
    await vi.advanceTimersByTimeAsync(0);
    expect(done).toHaveBeenCalled();
    expect(p.state()).toMatchObject({ url: B, playing: true });
  });

  it("resume during a pause fade takes it back and ramps up from where it is", async () => {
    const { p, audios, ctxs } = setup();
    p.play(A);
    await vi.advanceTimersByTimeAsync(2000);
    const g = ctxs[0].gain.gain;
    void p.pause();
    g.value = 0.5; // half way down
    p.resume();
    expect(g.last("set")).toEqual(["set", 0.5, 10]);
    expect(g.last("ramp")).toEqual(["ramp", 1, 10 + FADE_IN]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(audios[0].paused).toBe(false);
    expect(p.state().playing).toBe(true);
  });

  it("resume after the end replays from the start, onEnd once per play", async () => {
    const { p, audios } = setup();
    const end = vi.fn();
    p.onEnd(end);
    p.play(A, { from: 30, length: 20 });
    const el = audios[0];
    el.currentTime = 49.5;
    await vi.advanceTimersByTimeAsync(1000);
    expect(end).toHaveBeenCalledTimes(1);
    el.ended = true; // a late tick must not fire it again
    await vi.advanceTimersByTimeAsync(500);
    expect(end).toHaveBeenCalledTimes(1);
    el.ended = false;
    p.resume();
    expect(el.currentTime).toBe(30);
    expect(p.state()).toMatchObject({ playing: true, total: 20 });
    el.currentTime = 49.5;
    await vi.advanceTimersByTimeAsync(1000);
    expect(end).toHaveBeenCalledTimes(2);
  });

  it("switching engines releases the elements and starts over", () => {
    const { p, audios } = setup();
    const seen: string[] = [];
    p.onProgress((s) => seen.push(s.engine));
    p.play(A);
    p.prefetch(B);
    p.setEngine("webaudio");
    p.setEngine("element");
    expect(p.engine).toBe("element");
    for (const el of audios.slice(0, 2)) {
      expect(el.paused).toBe(true);
      expect(el.src).toBe("");
      expect(el.load).toHaveBeenCalled(); // stops the download too
    }
    expect(seen.at(-1)).toBe("element");
    p.play(A);
    expect(audios).toHaveLength(3);
  });
});

describe("PreviewPlayer, element engine", () => {
  it("steps the volume up over 1.5 s and down over 0.8 s", async () => {
    const f = fakeEnv();
    const p = new PreviewPlayer({ engine: "element", env: f.env });
    p.play(A);
    const el = f.audios[0];
    expect(f.ctxs).toHaveLength(0);
    expect(el.volume).toBe(0);
    await vi.advanceTimersByTimeAsync(750);
    expect(el.volume).toBeCloseTo(0.5, 1);
    await vi.advanceTimersByTimeAsync(800);
    expect(el.volume).toBe(1);
    expect(p.level()).toBe(1);
    void p.fadeOut();
    await vi.advanceTimersByTimeAsync(400);
    expect(el.volume).toBeCloseTo(0.5, 1);
    await vi.advanceTimersByTimeAsync(440);
    expect(el.volume).toBe(0);
    expect(el.paused).toBe(true);
  });

  it("plays without fades where volume is read-only (iOS)", async () => {
    const f = fakeEnv({ volumeWorks: false });
    const p = new PreviewPlayer({ engine: "element", env: f.env });
    p.play(A);
    const el = f.audios[0];
    expect(el.volume).toBe(1);
    await vi.advanceTimersByTimeAsync(2000);
    expect(el.volume).toBe(1);
    void p.fadeOut();
    await vi.advanceTimersByTimeAsync(850);
    expect(el.paused).toBe(true);
  });

  it("is what webaudio falls back to without Web Audio", () => {
    const f = fakeEnv({ webAudio: false });
    const p = new PreviewPlayer({ engine: "webaudio", env: f.env });
    p.play(A);
    expect(p.engine).toBe("element");
    expect(f.audios[0].volume).toBe(0);
  });
});

describe("helpers", () => {
  it("setAudioSession tolerates browsers without it or rejecting it", () => {
    expect(() => setAudioSession(undefined)).not.toThrow();
    expect(() => setAudioSession({} as Navigator)).not.toThrow();
    const nav = {
      audioSession: {
        set type(_: string) {
          throw new TypeError("unsupported");
        },
      },
    };
    expect(() => setAudioSession(nav as unknown as Navigator)).not.toThrow();
  });

  it("silentWav is a data URI of a RIFF/WAVE file", () => {
    const wav = silentWav();
    expect(wav.startsWith("data:audio/wav;base64,")).toBe(true);
    const bytes = atob(wav.split(",")[1]);
    expect(bytes.slice(0, 4)).toBe("RIFF");
    expect(bytes.slice(8, 12)).toBe("WAVE");
    expect(bytes).toHaveLength(52);
  });

  it("defaultEngine: query, then saved choice, then webaudio", () => {
    expect(defaultEngine()).toBe("webaudio");
    saveEngine("element");
    expect(defaultEngine()).toBe("element");
    window.history.replaceState(null, "", "/?audio=webaudio");
    expect(defaultEngine()).toBe("webaudio");
  });

  it("defaultPrefetch is on unless ?prefetch=off", () => {
    expect(defaultPrefetch()).toBe(true);
    window.history.replaceState(null, "", "/?prefetch=off");
    expect(defaultPrefetch()).toBe(false);
  });

  it("player is one shared instance", () => {
    expect(player()).toBe(player());
  });
});
