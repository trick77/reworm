import { act, renderHook } from "@testing-library/react";
import { fakeEnv } from "./fakes";
import { PreviewPlayer } from "./player";
import { usePlayer } from "./usePlayer";

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("usePlayer", () => {
  it("follows the clip's progress and end", async () => {
    const f = fakeEnv();
    const p = new PreviewPlayer({ engine: "element", env: f.env });
    const { result, unmount } = renderHook(() => usePlayer(p));
    expect(result.current).toMatchObject({
      playing: false,
      url: null,
      player: p,
    });
    act(() => p.play("https://audio.example/a.m4a", { length: 20 }));
    expect(result.current.playing).toBe(true);
    f.audios[0].currentTime = 4;
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(result.current.pos).toBe(4);
    f.audios[0].ended = true;
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(result.current.playing).toBe(false);
    unmount();
  });

  it("uses the shared player by default", () => {
    const { result } = renderHook(() => usePlayer());
    expect(result.current.player).toBeInstanceOf(PreviewPlayer);
  });
});
