import {
  ApiError,
  BACKOFF_MS,
  HttpApi,
  isStatus,
  type EventSourceLike,
} from "./client";
import type { RoomState } from "./types";

type Call = { url: string; method: string; body?: unknown };

function fetchStub(reply: (c: Call) => Response | Promise<Response>) {
  const calls: Call[] = [];
  const fetch = vi.fn(async (url: string, init?: RequestInit) => {
    const c: Call = {
      url,
      method: init?.method ?? "GET",
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    };
    calls.push(c);
    return reply(c);
  });
  return { fetch, calls };
}

class FakeES implements EventSourceLike {
  static all: FakeES[] = [];
  url: string;
  closed = false;
  private ls = new Map<string, ((e: MessageEvent) => void)[]>();
  constructor(url: string) {
    this.url = url;
    FakeES.all.push(this);
  }
  addEventListener(type: string, f: (e: MessageEvent) => void) {
    this.ls.set(type, [...(this.ls.get(type) ?? []), f]);
  }
  close() {
    this.closed = true;
  }
  fire(type: string, data?: unknown) {
    const e = {
      data: typeof data === "string" ? data : JSON.stringify(data),
    } as MessageEvent;
    (this.ls.get(type) ?? []).forEach((f) => f(e));
  }
}

const state = (phase: RoomState["phase"], n = 0): RoomState => ({
  code: "K7Q4",
  mode: "years",
  phase,
  hostId: "p1",
  toWin: 10,
  settings: {
    deck: { sets: [], categories: [], genres: [], from: 1900, to: 2025 },
    clip: { length: 20, start: "chorus" },
    difficulty: "medium",
  },
  players: [],
  round: n
    ? {
        n,
        startedAt: 0,
        endsAt: 0,
        clip: { previewUrl: "u", from: 0, length: 20 },
        buzz: null,
        revealVotes: [],
        judgeVotes: {},
        result: null,
      }
    : null,
  nextClip: null,
});

beforeEach(() => {
  FakeES.all = [];
});

describe("HttpApi calls", () => {
  it("sends JSON and parses JSON", async () => {
    const { fetch, calls } = fetchStub(() => Response.json({ code: "K7Q4" }));
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    expect(await api.createRoom({} as never)).toEqual({ code: "K7Q4" });
    expect(calls[0]).toMatchObject({
      url: "/api/rooms",
      method: "POST",
      body: {},
    });
  });

  it("turns error bodies into ApiError with the server's code", async () => {
    const { fetch } = fetchStub(() =>
      Response.json(
        { error: "room_full", message: "room is full" },
        { status: 409 },
      ),
    );
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    const e = await api
      .joinRoom("K7Q4", { name: "Mia", avatar: "mic" })
      .catch((x) => x);
    expect(e).toBeInstanceOf(ApiError);
    expect(e).toMatchObject({
      status: 409,
      code: "room_full",
      message: "room is full",
    });
    expect(isStatus(e, 409)).toBe(true);
    expect(isStatus(new Error("x"), 409)).toBe(false);
  });

  it("names a non-JSON error by its status", async () => {
    const { fetch } = fetchStub(
      () => new Response("<html>bad gateway", { status: 502 }),
    );
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    await expect(api.me()).rejects.toMatchObject({
      status: 502,
      code: "http_502",
    });
  });

  it("reports an unreachable server as status 0", async () => {
    const fetch = vi.fn(() => Promise.reject(new TypeError("Failed to fetch")));
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    await expect(api.decks()).rejects.toMatchObject({
      status: 0,
      code: "network",
    });
  });

  it("reads the version, and refuses a body without one", async () => {
    let body: unknown = { version: "1.2.3" };
    const { fetch } = fetchStub(() => Response.json(body));
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    expect(await api.version()).toBe("1.2.3");
    body = {};
    await expect(api.version()).rejects.toMatchObject({ code: "bad_response" });
  });

  it("maps every endpoint to the contract's path and method", async () => {
    const { fetch, calls } = fetchStub(
      () => new Response(null, { status: 204 }),
    );
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    await api.setup({ token: "t", name: "Jan", avatar: "vinyl" });
    await api.profiles();
    await api.switchProfile("p2");
    await api.updateMe({ avatar: "mic" });
    await api.devices();
    await api.removeDevice("d 1");
    await api.pair("invite");
    await api.claimPair({ code: "H4K9QZ" });
    await api.room("K7Q4");
    await api.host("K7Q4", "next");
    await api.kick("K7Q4", "pl1");
    await api.roomSettings("K7Q4", {} as never);
    await api.answer("K7Q4", 3, "later");
    await api.buzz("K7Q4", 3);
    await api.reveal("K7Q4", 3);
    await api.judge("K7Q4", 3, 2);
    await api.startTraining({} as never);
    await api.trainSession("t1");
    await api.trainAnswer("t1", { cardId: "c1", year: 1984 });
    await api.trainSummary("t1");
    await api.trainStats();
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual([
      "POST /api/setup",
      "GET /api/profiles",
      "POST /api/profiles/switch",
      "PATCH /api/me",
      "GET /api/devices",
      "DELETE /api/devices/d%201",
      "POST /api/pair",
      "POST /api/pair/claim",
      "GET /api/rooms/K7Q4",
      "POST /api/rooms/K7Q4/next",
      "POST /api/rooms/K7Q4/kick",
      "POST /api/rooms/K7Q4/settings",
      "POST /api/rooms/K7Q4/answer",
      "POST /api/rooms/K7Q4/buzz",
      "POST /api/rooms/K7Q4/reveal",
      "POST /api/rooms/K7Q4/judge",
      "POST /api/train/sessions",
      "GET /api/train/sessions/t1",
      "POST /api/train/sessions/t1/answer",
      "GET /api/train/sessions/t1/summary",
      "GET /api/train/stats",
    ]);
    expect(calls[12].body).toEqual({ round: 3, choice: "later" });
    expect(calls[15].body).toEqual({ round: 3, points: 2 });
    expect(api.coverUrl("a/b")).toBe("/api/covers/a%2Fb");
  });
});

describe("HttpApi.subscribeRoom", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("takes the snapshot, follows state events, and reconnects with a fresh snapshot", async () => {
    let snap = state("lobby");
    const { fetch, calls } = fetchStub(() => Response.json(snap));
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    const seen: string[] = [];
    const live: boolean[] = [];
    const stop = api.subscribeRoom("K7Q4", {
      onState: (s) => seen.push(`${s.phase}${s.round?.n ?? ""}`),
      onLive: (l) => live.push(l),
    });
    await vi.advanceTimersByTimeAsync(0);
    expect(seen).toEqual(["lobby"]);
    const es = FakeES.all[0];
    expect(es.url).toBe("/api/rooms/K7Q4/events");
    es.fire("open");
    es.fire("state", state("playing", 1));
    es.fire("state", "{garbled");
    expect(seen).toEqual(["lobby", "playing1"]);

    // the stream drops: closed at once, reopened after the backoff with a new snapshot
    snap = state("result", 1);
    es.fire("error");
    expect(es.closed).toBe(true);
    expect(live).toEqual([true, false]);
    await vi.advanceTimersByTimeAsync(BACKOFF_MS - 1);
    expect(FakeES.all).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(FakeES.all).toHaveLength(2);
    expect(calls.filter((c) => c.url === "/api/rooms/K7Q4")).toHaveLength(2);
    expect(seen.at(-1)).toBe("result1");

    // a second drop without an event in between waits twice as long
    FakeES.all[1].fire("error");
    await vi.advanceTimersByTimeAsync(BACKOFF_MS);
    expect(FakeES.all).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(BACKOFF_MS);
    expect(FakeES.all).toHaveLength(3);

    stop();
    expect(FakeES.all[2].closed).toBe(true);
    FakeES.all[2].fire("error");
    FakeES.all[2].fire("state", state("over"));
    await vi.advanceTimersByTimeAsync(20_000);
    expect(FakeES.all).toHaveLength(3);
    expect(seen.at(-1)).toBe("result1");
  });

  it("reports a room that is gone", async () => {
    const { fetch } = fetchStub(() =>
      Response.json({ error: "no_room" }, { status: 404 }),
    );
    const api = new HttpApi({ fetch, eventSource: (u) => new FakeES(u) });
    const onError = vi.fn();
    const stop = api.subscribeRoom("ZZZZ", { onState: vi.fn(), onError });
    await vi.advanceTimersByTimeAsync(0);
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({ status: 404 }),
    );
    stop();
  });
});
