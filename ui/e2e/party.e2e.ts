// Party mode end to end: the admin's iPad hosts, phones join by the QR's URL and play (D11, D14, D15).
import { expect, test, type Page } from "@playwright/test";
import {
  clip,
  closeDevices,
  expectPlaying,
  LABEL,
  party,
  round,
  roomState,
  settings,
  shot,
  waitPhase,
  type Pad,
} from "./helpers";

test.afterEach(closeDevices);

const PLAYERS: [string, string][] = [
  ["Mia", "Mic Drop"],
  ["Luca", "Strummy"],
  ["Sarah", "Disco Dot"],
];

// strip is a player's tile in the stage's player strip.
const strip = (stage: Page, name: string) =>
  stage.locator(".strip .p").filter({ hasText: name });

const score = (pad: Pad, pt: number) =>
  expect(pad.page.getByText(new RegExp(`^${pt} pt · #\\d$`))).toBeVisible();

test("years: locked out, fastest right answer, photo finish, standings, podium, rematch", async ({
  browser,
}) => {
  const { stage, code, pads } = await party(browser, PLAYERS);
  const [mia, luca, sarah] = pads;
  await settings(stage, code, { toWin: 2 });
  for (const p of pads)
    await expect(p.page.getByText(/· first to 2$/)).toBeVisible();
  await shot(stage, "years-lobby");
  await shot(mia.page, "pad-lobby");

  await stage.getByRole("button", { name: "Start game · 3 players" }).click();
  await expect(
    stage.getByText("Earlier · Spot on · Later — answer on your phone"),
  ).toBeVisible();
  let s = await waitPhase(stage, code, "playing");
  let r = round(s);
  expect(s.round!.n).toBe(1);
  // the stage plays the round's clip and spins the record
  await expect(stage.getByTestId("vinyl")).toHaveClass(/spin/);
  await expectPlaying(stage, r.song.id);
  await expect(mia.page.locator(".pad-year")).toHaveText(
    String(s.round!.proposedYear),
  );

  // a wrong answer: that phone is out, the stage shows ✕ and never what was answered
  await mia.page.getByRole("button", { name: LABEL[r.wrong] }).click();
  await expect(mia.page.getByText("Locked out")).toBeVisible();
  await expect(strip(stage, "Mia").getByLabel("locked out")).toBeVisible();
  await expect(stage.getByText("1 locked out")).toBeVisible();
  await expect(stage.locator(".strip")).not.toContainText(
    /earlier|later|spot on/i,
  );
  await shot(stage, "years-locked-out");
  await shot(mia.page, "pad-locked-out");

  // two right answers at once: the first takes the round, the other is in the photo finish (the server's 1 s
  // window). Luca taps (his request is on its way once the tap is dispatched); Sarah's phone sends the same
  // press straight from its session right after, since Luca's result may replace her buttons before a second
  // tap lands. Either may arrive first.
  const button = luca.page.getByRole("button", { name: LABEL[r.right] });
  await expect(button).toBeEnabled();
  await button.dispatchEvent("click");
  const press = await sarah.page.request.post(`/api/rooms/${code}/answer`, {
    data: { round: 1, choice: r.right },
  });
  expect(press.ok()).toBe(true);
  s = await waitPhase(stage, code, ["result", "over"]);
  const res = s.round!.result!;
  const points = r.right === "exact" ? 2 : 1;
  expect(res.points).toBe(points);
  expect(res.song).toMatchObject({ title: r.song.title, year: r.song.year });
  const winner = pads.find(
    (p) => s.players.find((x) => x.id === res.winnerId)?.name === p.name,
  )!;
  const second = winner === luca ? sarah : luca;
  expect(res.photoFinish).toHaveLength(1);
  if (s.phase === "result") {
    await expect(stage.getByText("Round 1 goes to")).toBeVisible();
    await expect(stage.getByText(`${winner.name} +${points}`)).toBeVisible();
    await expect(stage.locator(".photo")).toContainText(second.name);
    await expect(stage.getByText(r.song.title).first()).toBeVisible();
    // standings: the winner on top with the round's points, marked
    const top = stage.locator(".set-row").first();
    await expect(top).toContainText(winner.name);
    await expect(top).toContainText(`${points} ▲`);
    await shot(stage, "years-result-photo-finish");
  }
  await expect(winner.page.getByText("You got it!")).toBeVisible();
  await expect(
    second.page.getByText(`${winner.name} was faster`),
  ).toBeVisible();
  await expect(
    second.page.getByText(
      /Photo finish! You were right behind\.|You were \d\.\d\d s behind/,
    ),
  ).toBeVisible();
  await expect(mia.page.getByText(`${winner.name} was faster`)).toBeVisible();
  await score(winner, points);
  await shot(winner.page, "pad-won");
  await shot(second.page, "pad-photo-finish");

  // the round's winner plays on alone until first to 2
  for (let n = 2; s.phase !== "over"; n++) {
    expect(n).toBeLessThan(6);
    await stage.getByRole("button", { name: "Next round ›" }).click();
    s = await waitPhase(stage, code, "playing");
    expect(s.round!.n).toBe(n);
    r = round(s);
    await winner.page.getByRole("button", { name: LABEL[r.right] }).click();
    s = await waitPhase(stage, code, ["result", "over"]);
  }

  // game over: the podium on the stage, the rank on each phone
  await expect(stage.getByText(`${winner.name} wins!`)).toBeVisible();
  await expect(stage.locator('.podium [data-place="1"]')).toContainText(
    winner.name,
  );
  await expect(winner.page.getByText("You win!")).toBeVisible();
  await expect(mia.page.getByText("Game over")).toBeVisible();
  await shot(stage, "years-podium");
  await shot(winner.page, "pad-game-over");

  // rematch: back to the lobby, scores 0, rounds count from 1 again and every phone may answer
  await stage.getByRole("button", { name: "Rematch" }).click();
  await expect(
    stage.getByRole("button", { name: "Start game · 3 players" }),
  ).toBeVisible();
  for (const p of pads) await score(p, 0);
  await stage.getByRole("button", { name: "Start game · 3 players" }).click();
  s = await waitPhase(stage, code, "playing");
  expect(s.round!.n).toBe(1);
  r = round(s);
  // the winner pressed in the last game's round 1 too: a new round 1 is a new round
  const again = winner.page.getByRole("button", { name: LABEL[r.right] });
  await expect(again).toBeEnabled();
  await again.click();
  // scores from 0 again: a right "spot on" is 2 points, first to 2 at once (over); earlier/later is 1 (result)
  const exact = r.right === "exact";
  s = await waitPhase(stage, code, exact ? "over" : "result");
  expect(s.round!.result).toMatchObject({
    winnerId: res.winnerId,
    points: exact ? 2 : 1,
  });
  await expect(
    winner.page.getByText(exact ? "You win!" : "You got it!"),
  ).toBeVisible();
});

test("buzzer: buzz pauses, reveal by two, judging agrees or the room breaks the tie, nobody buzzes", async ({
  browser,
}) => {
  const { stage, code, pads } = await party(browser, PLAYERS);
  const [mia, luca, sarah] = pads;
  await stage.getByRole("button", { name: "Buzzer" }).click();
  for (const p of pads)
    await expect(
      p.page.getByText(/^Buzzer: say artist \+ title/),
    ).toBeVisible();
  await stage.getByRole("button", { name: "Start game · 3 players" }).click();
  await expect(stage.getByText("Know it? Buzz!")).toBeVisible();
  await expectPlaying(
    stage,
    round(await waitPhase(stage, code, "playing")).song.id,
  );
  const judge = (p: Pad, label: RegExp) =>
    p.page
      .getByRole("group", { name: "Judge" })
      .getByRole("button", { name: label })
      .click();

  // round 1: Mia buzzes, the stage stops the clip and asks her to say it
  await mia.page.getByRole("button", { name: "BUZZ" }).click();
  await expect(
    stage.getByText("Mia buzzed — say artist + title!"),
  ).toBeVisible();
  await expect(stage.getByTestId("vinyl")).not.toHaveClass(/spin/);
  await expect.poll(() => clip(stage)).toBeNull(); // faded out and paused
  await expect(mia.page.getByText("Say artist + title!")).toBeVisible();
  await expect(luca.page.getByText("Mia buzzed")).toBeVisible();
  let s = await roomState(stage, code);
  expect(s.phase).toBe("buzzed");
  expect(s.round!.result).toBeNull(); // the song stays secret until the reveal
  await shot(stage, "buzzer-buzzed");
  await shot(mia.page, "pad-say-it");

  // the buzzer and one other reveal: the song is out, judging starts
  await mia.page.getByRole("button", { name: "Reveal the song" }).click();
  await expect(stage.getByText("Reveal · 1 pressed")).toBeVisible();
  await luca.page.getByRole("button", { name: "Reveal the song" }).click();
  await expect(stage.getByText("Did Mia get it?")).toBeVisible();
  s = await roomState(stage, code);
  expect(s.phase).toBe("judging");
  const song1 = round(s).song;
  await expect(sarah.page.locator(".pad-song b").first()).toHaveText(
    song1.title,
  );
  await shot(stage, "buzzer-judging");
  await shot(sarah.page, "pad-judge");

  // buzzer + one other agree on 2
  await judge(mia, /artist \+ title/);
  await judge(luca, /artist \+ title/);
  s = await waitPhase(stage, code, "result");
  expect(s.round!.result).toMatchObject({
    points: 2,
    winnerId: s.players[0].id,
  });
  await expect(stage.getByText("Mia +2")).toBeVisible();
  await expect(stage.getByText("knew artist + title")).toBeVisible();
  await expect(mia.page.getByText("You got it!")).toBeVisible();
  await shot(stage, "buzzer-result");

  // round 2: Luca buzzes, Luca says 2, Sarah 1: Mia's vote breaks the tie (lower median: 1)
  await stage.getByRole("button", { name: "Next round ›" }).click();
  await waitPhase(stage, code, "playing");
  await luca.page.getByRole("button", { name: "BUZZ" }).click();
  await luca.page.getByRole("button", { name: "Reveal the song" }).click();
  await sarah.page.getByRole("button", { name: "Reveal the song" }).click();
  await waitPhase(stage, code, "judging");
  await judge(luca, /artist \+ title/);
  await judge(sarah, /one of them/);
  await expect(stage.getByText("Luca: 2")).toBeVisible();
  await expect(stage.getByText("Sarah: 1")).toBeVisible();
  expect((await roomState(stage, code)).phase).toBe("judging");
  await judge(mia, /one of them/);
  s = await waitPhase(stage, code, "result");
  expect(s.round!.result).toMatchObject({
    points: 1,
    winnerId: s.players[1].id,
  });
  await expect(stage.getByText("Luca +1")).toBeVisible();
  await expect(stage.getByText("knew one of them")).toBeVisible();
  await score(luca, 1);

  // round 3: nobody buzzes in 15 s: the reveal, no points
  await stage.getByRole("button", { name: "Next round ›" }).click();
  await waitPhase(stage, code, "playing");
  s = await waitPhase(stage, code, "result", 20_000);
  expect(s.round!.result).toMatchObject({ points: 0, winnerId: null });
  await expect(stage.getByText("Nobody got it")).toBeVisible();
  await expect(sarah.page.getByText("Nobody got it")).toBeVisible();
  await score(mia, 2);
  await score(sarah, 0);
  await shot(stage, "buzzer-nobody");
});

test("reconnect: a phone reloaded mid-round gets its player back from the server", async ({
  browser,
}) => {
  const { stage, code, pads } = await party(browser, PLAYERS.slice(0, 2));
  const [mia, luca] = pads;
  await stage.getByRole("button", { name: "Start game · 2 players" }).click();
  const s = await waitPhase(stage, code, "playing");
  const r = round(s);
  await luca.page.getByRole("button", { name: LABEL[r.wrong] }).click();
  await expect(luca.page.getByText("Locked out")).toBeVisible();

  // storage gone too: only the guest cookie and rooms/{code}/me know who this phone is
  for (const p of pads) {
    await p.page.evaluate(() => localStorage.clear());
    await p.page.reload();
    await expect(p.page.getByText(p.name, { exact: true })).toBeVisible();
  }
  await expect(luca.page.getByText("Locked out")).toBeVisible();
  await expect(mia.page.locator(".pad-year")).toHaveText(
    String(s.round!.proposedYear),
  );
  await expect(strip(stage, "Luca").getByLabel("locked out")).toBeVisible();
  await shot(mia.page, "pad-reconnected");
  await mia.page.getByRole("button", { name: LABEL[r.right] }).click();
  const done = await waitPhase(stage, code, "result");
  expect(done.round!.result!.winnerId).toBe(done.players[0].id);
  await expect(mia.page.getByText("You got it!")).toBeVisible();
  expect(done.players).toHaveLength(2);
});

test("kick: a removed phone is told so and cannot rejoin", async ({
  browser,
}) => {
  const { stage, code, pads } = await party(browser, PLAYERS.slice(0, 2));
  const luca = pads[1];
  await stage.getByRole("button", { name: "Remove Luca" }).click();
  await expect(stage.getByText("Players · 1")).toBeVisible();
  await expect(
    luca.page.getByText("You're not in this game any more. Join again?"),
  ).toBeVisible();
  await luca.page.getByRole("button", { name: "Join game" }).click();
  await expect(luca.page.getByRole("alert")).toHaveText(
    "The host removed you from this game.",
  );
  await luca.page.reload();
  await expect(luca.page.getByRole("alert")).toHaveText(
    "The host removed you from this game.",
  );
  await shot(luca.page, "pad-kicked");
  expect((await roomState(stage, code)).players.map((p) => p.name)).toEqual([
    "Mia",
  ]);
});
