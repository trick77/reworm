import { fireEvent, within } from "@testing-library/react";
import { audio } from "../../test/audio";
import { FakeWorld } from "../../test/fakeApi";
import { renderApp } from "../../test/render";

// Its own file: nothing has unlocked the audio in this module instance yet (a stage reloaded mid-game).
it("asks for one tap when the stage opens mid-game without sound, then plays the clip", async () => {
  const world = new FakeWorld();
  const host = world.device("p-jan");
  world.codes = ["K7Q4"];
  const { code } = await host.createRoom({
    mode: "years",
    deck: { sets: [], categories: [], genres: [], from: 1900, to: 2025 },
    toWin: 10,
    clip: { length: 20, start: "chorus" },
    difficulty: "medium",
  });
  world.addPlayer(code, "Mia", "mic");
  await host.host(code, "start");
  const r = renderApp(`/room/${code}`, host);
  const s = within(r.container);
  fireEvent.click(await s.findByRole("button", { name: "Turn sound on" }));
  expect(s.queryByText("Sound is off")).toBeNull();
  expect(s.getByText("0 locked out")).toBeInTheDocument();
  const url = world.room(code).state.round!.clip.previewUrl;
  expect(audio.audios.some((a) => a.src === url && !a.paused)).toBe(true);
});
