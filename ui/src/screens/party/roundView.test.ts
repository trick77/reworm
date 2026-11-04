import { describe, expect, it } from "vitest";
import type { RoomState } from "../../api/types";
import { roundView } from "./Stage";

// A buzzer round: Luca buzzed, the song is revealed, nobody judged yet; the host paused.
function paused(winnerId: string | null): RoomState {
  return {
    mode: "buzzer",
    phase: "paused",
    round: {
      n: 1,
      buzz: { playerId: "luca", at: 1, answerEndsAt: 2 },
      result: { winnerId },
    },
  } as unknown as RoomState;
}

describe("roundView while paused", () => {
  it("keeps the judging when this screen saw it", () => {
    expect(roundView(paused(null), "judging")).toBe("judging");
  });

  it("shows the judging when reveal and pause arrived in one batch", () => {
    // the screen last drew "buzzed": the reveal it never drew left a result in the round
    expect(roundView(paused(null), "buzzed")).toBe("judging");
    expect(roundView(paused(null), "playing")).toBe("judging");
  });

  it("keeps a finished round's result", () => {
    expect(roundView(paused("luca"), "result")).toBe("result");
    expect(roundView(paused(null), "result")).toBe("result");
  });

  it("stays live when no song was revealed yet", () => {
    const s = paused(null);
    s.round = { ...s.round!, result: null };
    expect(roundView(s, "buzzed")).toBe("live");
  });
});
