// Devices and party plumbing shared by the specs. Every device is its own browser context (own cookies and
// storage), as a phone or an iPad at a party would be.
import {
  expect,
  test,
  type Browser,
  type BrowserContext,
  type Page,
} from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Choice, RoomState } from "../src/api/types";
import { songOf, type FixtureSong } from "./songs";

const here = path.dirname(fileURLToPath(import.meta.url));

export const baseURL = () => process.env.E2E_BASE_URL!;
// the admin device's cookies, written by setup.e2e.ts
export const adminState = () => path.join(process.env.E2E_DIR!, "admin.json");

const SIZES = {
  stage: { viewport: { width: 1180, height: 820 }, deviceScaleFactor: 1 },
  phone: {
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    isMobile: true,
    hasTouch: true,
  },
  laptop: { viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 },
};

const open: BrowserContext[] = [];

// device opens a new browser (context) of a kind, the admin's paired one with admin: true.
export async function device(
  browser: Browser,
  kind: keyof typeof SIZES,
  opts: { admin?: boolean } = {},
): Promise<Page> {
  const ctx = await browser.newContext({
    baseURL: baseURL(),
    ...SIZES[kind],
    storageState: opts.admin ? adminState() : undefined,
  });
  open.push(ctx);
  return ctx.newPage();
}

// closeDevices ends every device of the test (each holds event streams: the server caps them per address).
export async function closeDevices() {
  await Promise.all(open.splice(0).map((c) => c.close()));
}

// shot saves a screenshot of a key moment (E2E_SHOTS, else test-results/shots).
export async function shot(page: Page, name: string) {
  const dir = process.env.E2E_SHOTS ?? path.join(here, "../test-results/shots");
  // the moment as it settles: entrance animations done (endless ones, the record's spin, keep going). At most
  // 1.5 s: a countdown ring animates its whole window, and the room's timers keep running meanwhile.
  await page
    .waitForFunction(
      () =>
        document
          .getAnimations()
          .every(
            (a) =>
              a.playState !== "running" ||
              a.effect?.getComputedTiming().iterations === Infinity,
          ),
      null,
      { timeout: 1_500 },
    )
    .catch(() => {});
  await page.screenshot({
    path: path.join(dir, `${test.info().project.name}-${name}.png`),
  });
}

export const LABEL: Record<Choice, string> = {
  earlier: "Earlier",
  exact: "Spot on",
  later: "Later",
};

// roomState is the room's snapshot, read with the device's cookies.
export async function roomState(page: Page, code: string): Promise<RoomState> {
  const r = await page.request.get(`/api/rooms/${code}`);
  expect(r.ok()).toBe(true);
  return (await r.json()) as RoomState;
}

// round is the running round's song (from its dev clip URL) and the right answer to its proposed year.
export function round(s: RoomState): {
  song: FixtureSong;
  right: Choice;
  wrong: Choice;
} {
  const r = s.round!;
  const song = songOf(r.clip.previewUrl);
  const shown = r.proposedYear;
  const right: Choice =
    shown == null || song.year === shown
      ? "exact"
      : song.year < shown
        ? "earlier"
        : "later";
  return { song, right, wrong: right === "later" ? "earlier" : "later" };
}

// waitPhase waits until the room is in phase (the stage's state, not a pad's).
export async function waitPhase(
  page: Page,
  code: string,
  phase: RoomState["phase"] | RoomState["phase"][],
  timeout = 10_000,
) {
  const want = Array.isArray(phase) ? phase : [phase];
  const seen: { s?: RoomState } = {};
  await expect
    .poll(
      async () => {
        const s = (seen.s = await roomState(page, code));
        return want.includes(s.phase) ? want[0] : s.phase;
      },
      { timeout, intervals: [200, 300, 500] },
    )
    .toBe(want[0]);
  return seen.s!;
}

type Media = { __media?: HTMLMediaElement[] };

// clip is what the stage's audio plays now: the playing element's song id and position, null when silent.
// The player's elements are not in the DOM; the stage's init script (hostRoom) collects them as they play.
export async function clip(stage: Page) {
  return stage.evaluate(() => {
    const el = ((window as unknown as Media).__media ?? []).findLast(
      (e) => !e.paused,
    );
    if (!el) return null;
    const m = /\/api\/dev\/previews\/([^/?]+)\.wav/.exec(el.currentSrc);
    return { id: m ? decodeURIComponent(m[1]) : "", at: el.currentTime };
  });
}

// expectPlaying waits until the stage plays the song's clip and it moves on.
export async function expectPlaying(stage: Page, id: string) {
  await expect
    .poll(async () => {
      const c = await clip(stage);
      return c && c.id === id && c.at > 0.3 ? id : JSON.stringify(c);
    })
    .toBe(id);
}

// hostRoom opens /host on the admin's iPad: a fresh room in the lobby.
export async function hostRoom(browser: Browser) {
  const stage = await device(browser, "stage", { admin: true });
  await stage.addInitScript(() => {
    const w = window as unknown as Media;
    const play = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      w.__media = [...(w.__media ?? []).filter((e) => e !== this), this];
      return play.call(this);
    };
  });
  await stage.goto("/host");
  const box = stage.locator('[aria-label="Room code"]');
  await expect(box).toHaveText(/^[A-Z0-9]{4}$/);
  const code = (await box.textContent())!;
  // the QR's address, what a phone's camera opens
  const join = await stage.locator("[data-qr]").getAttribute("data-qr");
  expect(join).toBe(`${baseURL()}/join/${code}`);
  return { stage, code, join: join! };
}

export type Pad = { name: string; page: Page };

// joinPhone opens the join URL on a new phone and joins with a name and an avatar.
export async function joinPhone(
  browser: Browser,
  join: string,
  name: string,
  avatar: string,
): Promise<Pad> {
  const page = await device(browser, "phone");
  await page.goto(join);
  await page.getByLabel("Your name").fill(name);
  await page.getByRole("radio", { name: avatar }).click();
  await page.getByRole("button", { name: "Join game" }).click();
  await expect(page.getByText("You're in!")).toBeVisible();
  return { name, page };
}

// party hosts a room and joins the given phones.
export async function party(browser: Browser, players: [string, string][]) {
  const room = await hostRoom(browser);
  const pads: Pad[] = [];
  for (const [name, avatar] of players)
    pads.push(await joinPhone(browser, room.join, name, avatar));
  await expect(
    room.stage.getByText(`Players · ${players.length}`),
  ).toBeVisible();
  return { ...room, pads };
}

// settings changes the lobby's settings through the host's settings call (the lobby UI offers first to 5,
// 10 or 15 only).
export async function settings(
  stage: Page,
  code: string,
  patch: Partial<{ toWin: number; mode: RoomState["mode"] }>,
) {
  const s = await roomState(stage, code);
  const r = await stage.request.post(`/api/rooms/${code}/settings`, {
    data: {
      mode: s.mode,
      toWin: s.toWin,
      deck: s.settings.deck,
      clip: s.settings.clip,
      difficulty: s.settings.difficulty,
      ...patch,
    },
  });
  expect(r.ok()).toBe(true);
}
