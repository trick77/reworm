import { describe, expect, it } from "vitest";
import { FakeWorld } from "../../test/fakeApi";
import { roundView } from "./Stage";

// A buzzer room on the fake server (which pauses like Room.Pause), round 1 running.
async function buzzerRound() {
  const world = new FakeWorld();
  const [, mia, luca] = world.demo("K7Q4", "buzzer");
  await world.hostAction("K7Q4", "start");
  return { world, mia, luca, state: () => world.room("K7Q4").state };
}

describe("roundView", () => {
  it("keeps the judging through a pause: the server says what the pause froze", async () => {
    const { world, mia, luca, state } = await buzzerRound();
    await world.buzzAs("K7Q4", luca, 1);
    await world.revealAs("K7Q4", luca, 1);
    await world.revealAs("K7Q4", mia, 1);
    expect(state().phase).toBe("judging");
    expect(roundView(state())).toBe("judging");
    await world.hostAction("K7Q4", "pause");
    expect(state().pausedFrom).toBe("judging");
    expect(roundView(state())).toBe("judging");
    await world.hostAction("K7Q4", "resume");
    expect(state().pausedFrom).toBeUndefined();
    expect(roundView(state())).toBe("judging");
  });

  it("stays live when a round is paused before the reveal", async () => {
    const { world, state } = await buzzerRound();
    await world.hostAction("K7Q4", "pause");
    expect(state().pausedFrom).toBe("playing");
    expect(roundView(state())).toBe("live");
  });
});
