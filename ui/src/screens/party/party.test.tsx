import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { audio } from "../../test/audio";
import { FakeWorld, type FakeApi } from "../../test/fakeApi";
import { renderApp } from "../../test/render";

// host opens /host on a paired device: the room is K7Q4, the stage shows the lobby
async function hostRoom(world: FakeWorld) {
  world.codes = ["K7Q4"];
  const stageApi = world.device("p-jan", "iPad");
  const stage = renderApp("/host", stageApi);
  const s = within(stage.container);
  expect(await s.findByLabelText("Room code")).toHaveTextContent("K7Q4");
  expect(stage.path()).toBe("/room/K7Q4");
  return { stageApi, stage, s };
}

async function joinPad(world: FakeWorld, name: string, avatar = "Woofer") {
  const padApi = world.device();
  const pad = renderApp("/join/K7Q4", padApi);
  const p = within(pad.container);
  fireEvent.change(p.getByLabelText("Your name"), { target: { value: name } });
  fireEvent.click(p.getByRole("radio", { name: avatar }));
  fireEvent.click(p.getByRole("button", { name: "Join game" }));
  expect(await p.findByText("You're in!")).toBeInTheDocument();
  return { padApi, pad, p, id: (padApi as FakeApi).playerId("K7Q4")! };
}

describe("party: lobby and join", () => {
  it("hosts a room, a phone joins with name and avatar, the lobby lists it", async () => {
    const world = new FakeWorld();
    const { s, stage } = await hostRoom(world);
    expect(s.getByText("Players · 0")).toBeInTheDocument();
    expect(s.getByRole("button", { name: /Start game/ })).toBeDisabled();
    expect(stage.container.querySelector("[data-qr]")).toHaveAttribute(
      "data-qr",
      `${window.location.origin}/join/K7Q4`,
    );

    const { p } = await joinPad(world, "Mia");
    expect(await s.findByText("Players · 1")).toBeInTheDocument();
    expect(s.getByText("Mia")).toBeInTheDocument();
    expect(p.getByText(/Waiting for the host/)).toHaveTextContent("1 player");
    expect(JSON.parse(localStorage.getItem("reworm.guest")!)).toEqual({
      name: "Mia",
      avatar: "speaker",
    });
  });

  it("types a code on /join, reports an unknown one, then joins and lands on /join/CODE", async () => {
    const world = new FakeWorld();
    await hostRoom(world);
    const pad = renderApp("/join", world.device());
    const p = within(pad.container);
    fireEvent.change(p.getByLabelText("Room code"), {
      target: { value: "zz-zz" },
    });
    fireEvent.change(p.getByLabelText("Your name"), {
      target: { value: "Tom" },
    });
    fireEvent.click(p.getByRole("button", { name: "Join game" }));
    expect(await p.findByRole("alert")).toHaveTextContent(
      "No game with the code ZZZZ",
    );
    fireEvent.change(p.getByLabelText("Room code"), {
      target: { value: "k7q4" },
    });
    fireEvent.keyDown(p.getByLabelText("Room code"), { key: "Enter" });
    expect(await p.findByText("You're in!")).toBeInTheDocument();
    expect(pad.path()).toBe("/join/K7Q4");
  });

  it("refuses a ninth player", async () => {
    const world = new FakeWorld();
    await hostRoom(world);
    for (let i = 0; i < 8; i++) world.addPlayer("K7Q4", `P${i}`, "vinyl");
    const pad = renderApp("/join/K7Q4", world.device());
    const p = within(pad.container);
    fireEvent.change(p.getByLabelText("Your name"), {
      target: { value: "Zoe" },
    });
    fireEvent.click(p.getByRole("button", { name: "Join game" }));
    expect(await p.findByRole("alert")).toHaveTextContent("full");
  });

  it("changes the game settings in the lobby and kicks a player", async () => {
    const world = new FakeWorld();
    const { s } = await hostRoom(world);
    const { p } = await joinPad(world, "Mia");
    await s.findByText("Players · 1");
    fireEvent.click(s.getByRole("button", { name: "Buzzer" }));
    await waitFor(() => expect(world.room("K7Q4").state.mode).toBe("buzzer"));
    expect(s.queryByRole("group", { name: "Difficulty" })).toBeNull();
    fireEvent.click(s.getByRole("button", { name: "15" }));
    fireEvent.click(s.getByRole("button", { name: "30 s" }));
    fireEvent.click(s.getByRole("button", { name: "Random" }));
    await waitFor(() =>
      expect(world.room("K7Q4").state.settings.clip).toEqual({
        length: 30,
        start: "random",
      }),
    );
    expect(world.room("K7Q4").state.toWin).toBe(15);
    fireEvent.click(s.getByRole("button", { name: "Change" }));
    fireEvent.click(await s.findByRole("button", { name: /Mega Mix/ }));
    await waitFor(() =>
      expect(world.room("K7Q4").state.settings.deck.sets).toEqual([
        "essentials",
        "megamix",
      ]),
    );
    fireEvent.click(s.getByRole("button", { name: "Earlier / Later" }));
    fireEvent.click(await s.findByRole("button", { name: "Hard" }));
    await waitFor(() =>
      expect(world.room("K7Q4").state.settings.difficulty).toBe("hard"),
    );
    expect(JSON.parse(localStorage.getItem("reworm.room")!).difficulty).toBe(
      "hard",
    );

    fireEvent.click(s.getByRole("button", { name: "Remove Mia" }));
    expect(await s.findByText("Players · 0")).toBeInTheDocument();
    // the pad finds itself gone and offers to join again
    expect(await p.findByText(/not in this game any more/)).toBeInTheDocument();
  });

  it("resumes the room this device hosts", async () => {
    const world = new FakeWorld();
    const { stage, stageApi } = await hostRoom(world);
    stage.unmount();
    const again = renderApp("/host", stageApi);
    await within(again.container).findByLabelText("Room code");
    expect(again.path()).toBe("/room/K7Q4");
    expect(world.rooms.size).toBe(1);
  });

  it("asks an unpaired device to pair before hosting, and reports a gone room", async () => {
    const world = new FakeWorld();
    const r = renderApp("/host", world.device());
    expect(
      await screen.findByText("This device isn't paired"),
    ).toBeInTheDocument();
    r.unmount();
    const g = renderApp("/room/GONE", world.device("p-jan"));
    expect(await screen.findByText("Room GONE is gone")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Host a new game" }));
    expect(g.path()).toBe("/host");
  });
});

describe("party: years mode", () => {
  it("plays the clip on the stage; a wrong answer locks the pad out; the right one takes the round", async () => {
    const world = new FakeWorld();
    const { s } = await hostRoom(world);
    const { p, id: mia } = await joinPad(world, "Mia");
    const luca = world.addPlayer("K7Q4", "Luca", "guitar");
    fireEvent.click(
      await s.findByRole("button", { name: "Start game · 2 players" }),
    );

    // the stage plays round 1's clip and preloads round 2's
    expect(
      await s.findByText("Earlier · Spot on · Later — answer on your phone"),
    ).toBeInTheDocument();
    const r = world.room("K7Q4").state.round!;
    expect(
      audio.audios.some((a) => a.src === r.clip.previewUrl && !a.paused),
    ).toBe(true);
    expect(
      audio.audios.some(
        (a) => a.src === world.room("K7Q4").state.nextClip!.previewUrl,
      ),
    ).toBe(true);
    expect(s.getByText(String(r.proposedYear))).toBeInTheDocument();

    // the pad: the year, three buttons
    expect(await p.findByText(String(r.proposedYear))).toBeInTheDocument();
    const right = world.correct("K7Q4");
    const wrong = right === "later" ? "Earlier" : "Later";
    fireEvent.click(p.getByRole("button", { name: wrong }));
    expect(await p.findByText("Locked out")).toBeInTheDocument();
    expect(await s.findByText("1 locked out")).toBeInTheDocument();
    expect(s.getByLabelText("locked out")).toBeInTheDocument();
    expect(
      world.room("K7Q4").state.players.find((x) => x.id === mia)!.state,
    ).toBe("out");

    world.set("K7Q4", {
      round: { ...world.room("K7Q4").state.round!, result: null },
    });
    await world.answerAs("K7Q4", luca, 1, right);
    expect(await s.findByText("Round 1 goes to")).toBeInTheDocument();
    expect(s.getByText(/fastest in the room|spot on/)).toBeInTheDocument();
    expect(await p.findByText("Luca was faster")).toBeInTheDocument();
    expect(p.getByText(world.song("K7Q4")!.title)).toBeInTheDocument();

    // next round: the stage plays the next clip
    fireEvent.click(s.getByRole("button", { name: "Next round ›" }));
    await waitFor(() => expect(world.room("K7Q4").state.round!.n).toBe(2));
    expect(await p.findByRole("button", { name: "Earlier" })).toBeEnabled();
    expect(
      audio.audios.some(
        (a) =>
          a.src === world.room("K7Q4").state.round!.clip.previewUrl &&
          !a.paused,
      ),
    ).toBe(true);
  });

  it("shows you got it, the photo finish, pause, nobody, game over and rematch", async () => {
    const world = new FakeWorld();
    const { s } = await hostRoom(world);
    const { p, id: mia } = await joinPad(world, "Mia");
    const luca = world.addPlayer("K7Q4", "Luca", "guitar");
    fireEvent.click(await s.findByRole("button", { name: /Start game/ }));
    await p.findByRole("button", { name: "Earlier" });
    const label = {
      later: "Later",
      earlier: "Earlier",
      exact: "Spot on",
    } as const;
    const yes = p.getByRole("button", { name: label[world.correct("K7Q4")] });
    // pause and resume from the stage
    fireEvent.click(s.getByRole("button", { name: "Pause" }));
    expect(await s.findByText("Paused")).toBeInTheDocument();
    expect(await p.findByText("Paused by the host")).toBeInTheDocument();
    fireEvent.click(s.getAllByRole("button", { name: "Resume" })[0]);
    await waitFor(() => expect(world.room("K7Q4").state.phase).toBe("playing"));
    fireEvent.click(yes);
    expect(await p.findByText("You got it!")).toBeInTheDocument();
    world.set("K7Q4", {
      round: {
        ...world.room("K7Q4").state.round!,
        result: {
          ...world.room("K7Q4").state.round!.result!,
          photoFinish: [{ playerId: luca, deltaMs: 30 }],
        },
      },
    });
    expect(await s.findByText("Photo finish!")).toBeInTheDocument();
    expect(s.getByText(/Luca photo finish/)).toBeInTheDocument();

    // nobody: all out
    fireEvent.click(s.getByRole("button", { name: "Next round ›" }));
    await waitFor(() => expect(world.room("K7Q4").state.round!.n).toBe(2));
    const wrong = world.correct("K7Q4") === "later" ? "earlier" : "later";
    await world.answerAs("K7Q4", mia, 2, wrong);
    await world.answerAs("K7Q4", luca, 2, wrong);
    expect(await s.findByText("Nobody got it")).toBeInTheDocument();
    expect(await p.findByText("Nobody got it")).toBeInTheDocument();

    // someone faster by a hair, as seen on the pad
    world.set("K7Q4", {
      round: {
        ...world.room("K7Q4").state.round!,
        result: {
          ...world.room("K7Q4").state.round!.result!,
          winnerId: luca,
          points: 1,
          photoFinish: [{ playerId: mia, deltaMs: 180 }],
        },
      },
    });
    expect(await p.findByText("You were 0.18 s behind")).toBeInTheDocument();

    // game over
    world.room("K7Q4").state.players.find((x) => x.id === luca)!.score = 10;
    world.set("K7Q4", { phase: "over" });
    expect(await s.findByText("Luca wins!")).toBeInTheDocument();
    expect(await p.findByText("You finished 2nd")).toBeInTheDocument();
    fireEvent.click(s.getByRole("button", { name: "Rematch" }));
    await waitFor(() => expect(world.room("K7Q4").state.phase).toBe("playing"));
    expect(world.room("K7Q4").state.players.every((x) => x.score === 0)).toBe(
      true,
    );
  });
});

describe("party: late join", () => {
  it("a phone joining mid-round sits it out, then plays the next one", async () => {
    const world = new FakeWorld();
    const { s } = await hostRoom(world);
    world.addPlayer("K7Q4", "Luca", "guitar");
    fireEvent.click(await s.findByRole("button", { name: /Start game/ }));
    await s.findByText("0 locked out");
    const padApi = world.device();
    const pad = renderApp("/join/K7Q4", padApi);
    const p = within(pad.container);
    fireEvent.change(p.getByLabelText("Your name"), {
      target: { value: "Zoe" },
    });
    fireEvent.click(p.getByRole("button", { name: "Join game" }));
    expect(
      await p.findByText("You're in from the next round"),
    ).toBeInTheDocument();
    world.endRound("K7Q4", null, 0);
    await world.hostAction("K7Q4", "next");
    expect(await p.findByRole("button", { name: "Earlier" })).toBeEnabled();
  });
});

describe("party: buzzer mode", () => {
  it("buzz, say it, reveal by two, judge by two, points to the buzzer", async () => {
    const world = new FakeWorld();
    const { s } = await hostRoom(world);
    fireEvent.click(s.getByRole("button", { name: "Buzzer" }));
    await waitFor(() => expect(world.room("K7Q4").state.mode).toBe("buzzer"));
    const { p } = await joinPad(world, "Mia");
    const luca = world.addPlayer("K7Q4", "Luca", "guitar");
    fireEvent.click(await s.findByRole("button", { name: /Start game/ }));
    expect(await s.findByText("Know it? Buzz!")).toBeInTheDocument();

    fireEvent.click(await p.findByRole("button", { name: "BUZZ" }));
    expect(await p.findByText("Say artist + title!")).toBeInTheDocument();
    expect(
      await s.findByText("Mia buzzed — say artist + title!"),
    ).toBeInTheDocument();
    // the clip fades out on the buzz
    await act(() => new Promise((r) => setTimeout(r, 900)));
    expect(audio.audios.every((a) => a.paused || a.muted)).toBe(true);

    fireEvent.click(p.getByRole("button", { name: "Reveal the song" }));
    expect(
      await p.findByRole("button", { name: "Waiting for one more Reveal…" }),
    ).toBeDisabled();
    expect(await s.findByText("Reveal · 1 pressed")).toBeInTheDocument();
    await world.revealAs("K7Q4", luca, 1);
    expect(await s.findByText("Did Mia get it?")).toBeInTheDocument();
    expect(s.getByText(world.song("K7Q4")!.title)).toBeInTheDocument();
    expect(await p.findByText("How did you do?")).toBeInTheDocument();

    fireEvent.click(
      within(p.getByRole("group", { name: "Judge" })).getByRole("button", {
        name: /artist \+ title/,
      }),
    );
    expect(
      await p.findByText("Waiting for the others to judge…"),
    ).toBeInTheDocument();
    expect(await s.findByText("Mia: 2")).toBeInTheDocument();
    await world.judgeAs("K7Q4", luca, 1, 2);
    expect(await p.findByText("You got it!")).toBeInTheDocument();
    expect(p.getByText("+2")).toBeInTheDocument();
    expect(await s.findByText("knew artist + title")).toBeInTheDocument();
  });

  it("shows another player's buzz on the pad and judges it", async () => {
    const world = new FakeWorld();
    const { s } = await hostRoom(world);
    fireEvent.click(s.getByRole("button", { name: "Buzzer" }));
    await waitFor(() => expect(world.room("K7Q4").state.mode).toBe("buzzer"));
    const { p } = await joinPad(world, "Mia");
    const luca = world.addPlayer("K7Q4", "Luca", "guitar");
    fireEvent.click(await s.findByRole("button", { name: /Start game/ }));
    await p.findByRole("button", { name: "BUZZ" });
    await world.buzzAs("K7Q4", luca, 1);
    expect(await p.findByText("Luca buzzed")).toBeInTheDocument();
    await world.revealAs("K7Q4", luca, 1);
    fireEvent.click(p.getByRole("button", { name: "Reveal the song" }));
    expect(await p.findByText("How did Luca do?")).toBeInTheDocument();
    fireEvent.click(p.getByRole("button", { name: /neither/ }));
    await world.judgeAs("K7Q4", luca, 1, 0);
    expect(await p.findByText("Nobody got it")).toBeInTheDocument();
    expect(await s.findByText("No points this round")).toBeInTheDocument();
  });
});
