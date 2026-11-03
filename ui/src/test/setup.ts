import "@testing-library/jest-dom/vitest";
// vitest setupFiles entry (vite.config.ts `test.setupFiles`): jest-dom's matchers on vitest's `expect`.
// Testing Library's cleanup runs by itself between tests (test.globals).

// The app's shared preview player plays on fake audio elements (jsdom has no media playback).
vi.mock("../audio/player", async (orig) => {
  const mod = await orig<typeof import("../audio/player")>();
  const { audio } = await import("./audio");
  let p: InstanceType<typeof mod.PreviewPlayer> | null = null;
  return {
    ...mod,
    player: () =>
      (p ??= new mod.PreviewPlayer({
        engine: "webaudio",
        prefetch: mod.defaultPrefetch(),
        env: audio.env,
      })),
    savePrefetch: (on: boolean) => {
      mod.savePrefetch(on);
      p?.dispose();
      p = null;
    },
  };
});

afterEach(() => {
  window.localStorage.clear();
});
