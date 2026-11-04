// Training: a session on the admin's laptop, answered by swipe, keys and buttons; the reveal, the summary, a
// drill of the misses.
import { expect, test, type Page } from "@playwright/test";
import type { Choice, TrainSession } from "../src/api/types";
import { closeDevices, device, LABEL, shot } from "./helpers";
import { songOf, type FixtureSong } from "./songs";

test.afterEach(closeDevices);

// card is the session's current card: its song (from the dev clip URL) and the right answer.
async function card(page: Page, id: string) {
  const r = await page.request.get(`/api/train/sessions/${id}`);
  expect(r.ok()).toBe(true);
  const s = (await r.json()) as TrainSession;
  const c = s.card!;
  const song = songOf(c.clip.previewUrl);
  const p = c.proposedYear!;
  const right: Choice =
    song.year === p ? "exact" : song.year < p ? "earlier" : "later";
  return {
    s,
    song,
    right,
    wrong: (right === "later" ? "earlier" : "later") as Choice,
  };
}

// swipe drags the year card: left earlier, up spot on, right later.
async function swipe(page: Page, c: Choice) {
  const box = (await page.getByTestId("yearcard").boundingBox())!;
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  const [dx, dy] =
    c === "earlier" ? [-160, 0] : c === "later" ? [160, 0] : [0, -160];
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx, y + dy, { steps: 8 });
  await page.mouse.up();
}

const KEY: Record<Choice, string> = {
  earlier: "ArrowLeft",
  exact: "ArrowUp",
  later: "ArrowRight",
};

async function revealed(page: Page, song: FixtureSong) {
  await expect(page.getByText(`First released ${song.year}`)).toBeVisible();
  await expect(page.getByText(song.title).first()).toBeVisible();
}

test("training: swipe, keys and buttons, the reveal, the summary, drilling the misses", async ({
  browser,
}) => {
  const page = await device(browser, "laptop", { admin: true });
  await page.goto("/train");
  await page.getByRole("button", { name: "Start · 10 songs" }).click();
  await expect(page).toHaveURL(/\/train\/[^/]+$/);
  const id = decodeURIComponent(page.url().split("/").pop()!);
  await expect(page.getByLabel("Song 1 of 10")).toBeVisible();

  // 1: right, by swipe
  let c = await card(page, id);
  expect(c.s.stats).toMatchObject({ answered: 0, correct: 0 });
  await expect(page.getByTestId("yearcard")).toContainText(
    String(c.s.card!.proposedYear),
  );
  await shot(page, "train-card");
  await swipe(page, c.right);
  await revealed(page, c.song);
  await expect(page.getByText(/^\+\d+ XP/)).toBeVisible();
  await shot(page, "train-reveal-right");
  await page.keyboard.press("Enter");

  // 2: wrong, by key: it comes back a few cards later
  await expect(page.getByLabel("Song 2 of 10")).toBeVisible();
  c = await card(page, id);
  const missed = c.song;
  await page.keyboard.press(KEY[c.wrong]);
  await revealed(page, c.song);
  await expect(
    page.getByText("Back in a few cards with a different year."),
  ).toBeVisible();
  await expect(page.getByLabel("Song 2 of 11")).toBeVisible();
  await shot(page, "train-reveal-miss");
  await page.getByRole("button", { name: /Next song/ }).click();

  // 3: right, by the button row
  await expect(page.getByLabel("Song 3 of 11")).toBeVisible();
  c = await card(page, id);
  await page
    .getByRole("button", { name: new RegExp(`^${LABEL[c.right]}`) })
    .click();
  await revealed(page, c.song);
  await page.keyboard.press("Enter");
  await expect(page.getByLabel("Song 4 of 11")).toBeVisible();
  c = await card(page, id);
  expect(c.s.stats).toMatchObject({ answered: 3, correct: 2 });

  // end early: the summary counts what was answered
  await page.getByRole("button", { name: "End session" }).click();
  await expect(page.getByText("Session complete")).toBeVisible();
  await expect(page.getByText("2 of 3 right")).toBeVisible();
  await expect(page.getByText("To learn (1)")).toBeVisible();
  await expect(page.locator(".miss")).toContainText(missed.title);
  await shot(page, "train-summary");

  // drill the misses: a new session of just that song
  await page.getByRole("button", { name: "Drill misses (1)" }).click();
  await expect(page).not.toHaveURL(new RegExp(`/train/${id}$`));
  await expect(page.getByLabel("Song 1 of 1")).toBeVisible();
  const drill = decodeURIComponent(page.url().split("/").pop()!);
  expect((await card(page, drill)).song.id).toBe(missed.id);
});
