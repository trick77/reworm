// The API client: every endpoint of the contract behind one interface, so screens and tests can swap the
// network for a fake. HttpApi is the real one: fetch for the calls, EventSource for the live room state.
import { BASE, withBase } from "../base";
import { syncServerClock } from "./clock";
import type {
  Choice,
  Decks,
  Device,
  Me,
  PairCode,
  Points,
  RoomSettings,
  RoomState,
  TrainAnswer,
  TrainSession,
  TrainSettings,
  TrainStarted,
  TrainStats,
  TrainSummary,
} from "./types";

// ApiError is a failed call: the HTTP status (0: no answer at all) and the server's error code.
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export type HostAction =
  "start" | "next" | "pause" | "resume" | "rematch" | "abort";

export type RoomHandlers = {
  onState: (s: RoomState) => void;
  // onError: the snapshot could not be fetched. A 404 (the room is gone) ends the subscription.
  onError?: (e: ApiError) => void;
  // onLive: the event stream is connected (true) or dropped and reconnecting (false)
  onLive?: (live: boolean) => void;
};

export type Leaving = { leave?: boolean };

// RoundRef names the round a press is for: numbers restart with every game, the start tells a late press from an
// ended game apart from the next game's round of the same number.
export type RoundRef = { n: number; startedAt: number };

export interface Api {
  version(): Promise<string>;
  me(): Promise<Me>;
  setup(body: { token: string; name: string; avatar: string }): Promise<void>;
  // createProfile makes a new profile for this device (anyone may, once the server is set up). leave: this
  // device may leave its profile although it is that profile's last one (409 last_device otherwise); the old
  // profile is deleted. claimPair takes leave the same way.
  createProfile(
    body: Leaving & { name: string; avatar: string },
  ): Promise<void>;
  updateMe(body: { name?: string; avatar?: string }): Promise<void>;
  devices(): Promise<Device[]>;
  removeDevice(id: string): Promise<void>;
  // pair makes a code that puts another device on this profile.
  pair(): Promise<PairCode>;
  claimPair(body: Leaving & { code: string }): Promise<void>;
  decks(): Promise<Decks>;

  createRoom(settings: RoomSettings): Promise<{ code: string }>;
  room(code: string): Promise<RoomState>;
  // subscribeRoom fetches the snapshot, then follows the live state; returns the unsubscribe.
  subscribeRoom(code: string, h: RoomHandlers): () => void;
  joinRoom(
    code: string,
    body: { name: string; avatar: string },
  ): Promise<{ playerId: string }>;
  // roomMe: this browser's player in the room (guest cookie or profile); null when it has none or the
  // room is gone. A kicked player is refused (403 kicked).
  roomMe(code: string): Promise<string | null>;
  // roomHost: whether this browser hosts the room (its host cookie, which asking renews).
  roomHost(code: string): Promise<boolean>;
  host(code: string, action: HostAction): Promise<void>;
  kick(code: string, playerId: string): Promise<void>;
  roomSettings(code: string, settings: RoomSettings): Promise<void>;
  answer(code: string, round: RoundRef, choice: Choice): Promise<void>;
  buzz(code: string, round: RoundRef): Promise<void>;
  reveal(code: string, round: RoundRef): Promise<void>;
  judge(code: string, round: RoundRef, points: Points): Promise<void>;

  startTraining(s: TrainSettings): Promise<TrainStarted>;
  trainSession(id: string): Promise<TrainSession>;
  trainAnswer(
    id: string,
    body: { cardId: string; choice?: Choice; year?: number },
  ): Promise<TrainAnswer>;
  trainSummary(id: string): Promise<TrainSummary>;
  trainStats(): Promise<TrainStats>;
}

// EventSourceLike is the part of EventSource the room subscription uses.
export type EventSourceLike = {
  addEventListener(type: string, f: (e: MessageEvent) => void): void;
  close(): void;
};

export type HttpEnv = {
  fetch: (input: string, init?: RequestInit) => Promise<Response>;
  eventSource: (url: string) => EventSourceLike;
};

// Retry backoff (a dropped event stream, a failed ask): 1 s, 2 s, 4 s … up to 10 s.
export const BACKOFF_MS = 1000;
export const BACKOFF_MAX_MS = 10_000;

// backoff is the wait before retry number `attempt` (0, 1, 2 …).
export const backoff = (attempt: number) =>
  Math.min(BACKOFF_MAX_MS, BACKOFF_MS * 2 ** attempt);

// transient: a failure worth retrying as it is (no answer, the server's own trouble, a rate limit).
export const transient = (e: ApiError) =>
  e.status === 0 || e.status === 429 || e.status >= 500;

// timeZone: the IANA zone of this device ("Europe/Zurich"), UTC when the browser can't tell.
export function timeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

const room = (code: string) => `/api/rooms/${encodeURIComponent(code)}`;
// press is a press body's round: its number and start (the server refuses another game's round of that number)
const press = (r: RoundRef) => ({ round: r.n, startedAt: r.startedAt });

// browserEnv is the real network, every app path under the base path (PUBLIC_PATH).
export function browserEnv(base: string = BASE): HttpEnv {
  return {
    fetch: (input, init) => window.fetch(withBase(input, base), init),
    eventSource: (url) =>
      new EventSource(withBase(url, base), { withCredentials: true }),
  };
}

export class HttpApi implements Api {
  private env: HttpEnv;
  constructor(env: Partial<HttpEnv> = {}) {
    this.env = { ...browserEnv(), ...env };
  }

  private async call<T>(
    method: string,
    path: string,
    body?: unknown,
    extra?: Record<string, string>,
    signal?: AbortSignal,
  ) {
    let res: Response;
    const headers: Record<string, string> = { ...extra };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    try {
      res = await this.env.fetch(path, {
        method,
        credentials: "same-origin",
        headers: Object.keys(headers).length ? headers : undefined,
        body: body === undefined ? undefined : JSON.stringify(body),
        ...(signal ? { signal } : {}),
      });
    } catch (e) {
      throw new ApiError(0, "network", (e as Error).message || "offline");
    }
    let text: string;
    try {
      text = await res.text();
    } catch (e) {
      throw new ApiError(0, "network", (e as Error).message || "cut off");
    }
    let data: unknown = undefined;
    try {
      data = text ? JSON.parse(text) : undefined;
    } catch {
      // not JSON: an error page from a proxy, or an empty success
    }
    if (!res.ok) {
      const err = (data ?? {}) as { error?: string; message?: string };
      throw new ApiError(
        res.status,
        err.error ?? `http_${res.status}`,
        // kit's own refusals (400, 413, 429) carry the code only
        err.message ??
          (res.status === 429
            ? "Too many tries. Wait a moment and try again."
            : `${method} ${path}: ${res.status}`),
      );
    }
    return data as T;
  }

  private get = <T>(path: string) => this.call<T>("GET", path);
  private post = <T = void>(path: string, body: unknown = {}) =>
    this.call<T>("POST", path, body);

  async version() {
    const v = await this.get<{ version?: unknown }>("/api/version");
    if (typeof v?.version !== "string")
      throw new ApiError(200, "bad_response", "no version in the response");
    return v.version;
  }
  me = () => this.get<Me>("/api/me");
  setup = (b: { token: string; name: string; avatar: string }) =>
    this.post("/api/setup", b);
  createProfile = (b: Leaving & { name: string; avatar: string }) =>
    this.post("/api/profiles", b);
  updateMe = (b: { name?: string; avatar?: string }) =>
    this.call<void>("PATCH", "/api/me", b);
  devices = () => this.get<Device[]>("/api/devices");
  removeDevice = (id: string) =>
    this.call<void>("DELETE", `/api/devices/${encodeURIComponent(id)}`);
  pair = () => this.post<PairCode>("/api/pair", { kind: "device" });
  claimPair = (b: Leaving & { code: string }) =>
    this.post("/api/pair/claim", b);
  decks = () => this.get<Decks>("/api/decks");

  createRoom = (s: RoomSettings) =>
    this.post<{ code: string }>("/api/rooms", s);
  room = (code: string) => this.get<RoomState>(room(code));
  joinRoom = (code: string, b: { name: string; avatar: string }) =>
    this.post<{ playerId: string }>(`${room(code)}/join`, b);
  roomMe = async (code: string) => {
    try {
      const r = await this.get<{ playerId?: string } | undefined>(
        `${room(code)}/me`,
      );
      return r?.playerId ?? null;
    } catch (e) {
      if (e instanceof ApiError && (e.status === 401 || e.status === 404))
        return null;
      throw e;
    }
  };
  roomHost = async (code: string) =>
    (await this.get<{ host?: boolean } | undefined>(`${room(code)}/host`))
      ?.host === true;
  host = (code: string, action: HostAction) =>
    this.post(`${room(code)}/${action}`);
  kick = (code: string, playerId: string) =>
    this.post(`${room(code)}/kick`, { playerId });
  roomSettings = (code: string, s: RoomSettings) =>
    this.post(`${room(code)}/settings`, s);
  answer = async (code: string, r: RoundRef, choice: Choice) => {
    await this.post(`${room(code)}/answer`, { ...press(r), choice });
  };
  buzz = (code: string, r: RoundRef) =>
    this.post(`${room(code)}/buzz`, press(r));
  reveal = (code: string, r: RoundRef) =>
    this.post(`${room(code)}/reveal`, press(r));
  judge = (code: string, r: RoundRef, points: Points) =>
    this.post(`${room(code)}/judge`, { ...press(r), points });

  // training calls carry the device's time zone: the server counts streak days in it
  private train = <T>(method: string, path: string, body?: unknown) =>
    this.call<T>(method, path, body, { "X-Timezone": timeZone() });
  startTraining = (s: TrainSettings) =>
    this.train<TrainStarted>("POST", "/api/train/sessions", s);
  trainSession = (id: string) =>
    this.train<TrainSession>(
      "GET",
      `/api/train/sessions/${encodeURIComponent(id)}`,
    );
  trainAnswer = (
    id: string,
    b: { cardId: string; choice?: Choice; year?: number },
  ) =>
    this.train<TrainAnswer>(
      "POST",
      `/api/train/sessions/${encodeURIComponent(id)}/answer`,
      b,
    );
  trainSummary = (id: string) =>
    this.train<TrainSummary>(
      "GET",
      `/api/train/sessions/${encodeURIComponent(id)}/summary`,
    );
  trainStats = () => this.train<TrainStats>("GET", "/api/train/stats");

  // wait is one long poll: the room's state once it is newer than seq `after` (none: at once), else the
  // unchanged state after the server's hold.
  // `cut` names the stream the page gave up on: the server ends it (the proxy holds it open there).
  private wait = (
    code: string,
    after: number,
    cut: string,
    signal: AbortSignal,
  ) => {
    const q = new URLSearchParams();
    if (Number.isFinite(after)) q.set("after", String(after));
    if (cut) q.set("cut", cut);
    const qs = q.toString();
    return this.call<RoomState>(
      "GET",
      `${room(code)}/wait${qs ? `?${qs}` : ""}`,
      undefined,
      undefined,
      signal,
    );
  };

  // subscribeRoom: snapshot first, then the event stream. A dropped stream is closed (EventSource's own
  // retry would skip what changed meanwhile) and reopened after a backoff, with a fresh snapshot.
  //
  // A proxy that buffers the response (TLS inspection in an office) passes the stream's 200 and then
  // nothing, or everything late: the stream is stalled when its first state misses PROBE_MS, when nothing
  // (state or ping) comes for WATCHDOG_MS, or when LAG_EVENTS states in a row come LAG_MS late. Then the
  // subscription long-polls (/wait, whole responses any proxy passes) for good; its first poll names the
  // stalled stream (`sid`) for the server to end. A phone that slept looks the same for a moment (timers fire
  // late, queued states arrive old): while the page is hidden, a timer fired late, or within RESUME_MS of
  // coming back, a stall only reconnects the stream.
  subscribeRoom(code: string, h: RoomHandlers): () => void {
    let closed = false;
    let es: EventSourceLike | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let watch: ReturnType<typeof setTimeout> | undefined;
    let poller: AbortController | null = null;
    let attempt = 0;
    let resumedAt = -Infinity;
    const onVisible = () => {
      if (!document.hidden) resumedAt = Date.now();
    };
    document.addEventListener("visibilitychange", onVisible);
    const settling = () =>
      document.hidden || Date.now() - resumedAt < RESUME_MS;
    let live: boolean | undefined;
    const setLive = (l: boolean) => {
      if (l === live) return;
      live = l;
      h.onLive?.(l);
    };
    // order: a state older than the last applied one (by seq) is dropped, snapshot or event. One of the same
    // seq changes nothing but still sets the server clock (`now`), unless it came late.
    let lastSeq = -Infinity;
    const apply = (s: RoomState, clock = true) => {
      if (s.seq < lastSeq) return;
      if (clock) syncServerClock(s.now);
      if (s.seq === lastSeq) return;
      lastSeq = s.seq;
      h.onState(s);
    };
    // lag: a whole response's `now` sets the reference offset (the least delayed one seen); an event whose
    // offset falls LAG_MS behind it sat in a buffer on the way.
    let ref = -Infinity;
    let late = 0;
    const sample = (s: RoomState) => {
      ref = Math.max(ref, s.now - Date.now());
    };
    const stop = () => {
      closed = true;
      clearTimeout(timer);
      clearTimeout(watch);
      es?.close();
      es = null;
      poller?.abort();
      poller = null;
      document.removeEventListener("visibilitychange", onVisible);
    };

    const snapshot = () =>
      this.room(code).then(
        (s) => {
          if (closed) return;
          sample(s);
          apply(s);
        },
        (e: ApiError) => {
          if (closed) return;
          if (e.status === 404) stop(); // the room is gone: nothing to reconnect to
          h.onError?.(e);
        },
      );

    // poll: the first request goes without `after`, answered at once (a fresh snapshot, and live again without
    // waiting out a quiet room's hold). `cut` rides along until a poll is answered. A refusal that is no outage
    // (not transient, not 404) is reported once per run of failures.
    const poll = async (cut: string) => {
      let first = true;
      let reported = false;
      while (!closed) {
        const ac = new AbortController();
        poller = ac;
        try {
          const s = await this.wait(
            code,
            first ? -Infinity : lastSeq,
            cut,
            ac.signal,
          );
          if (closed) return;
          first = false;
          cut = "";
          attempt = 0;
          reported = false;
          setLive(true);
          sample(s);
          apply(s);
        } catch (e) {
          if (closed) return;
          const err = e as ApiError;
          if (err.status === 404) {
            stop(); // the room is gone: nothing to poll
            h.onError?.(err);
            return;
          }
          if (!transient(err) && !reported) {
            reported = true;
            h.onError?.(err);
          }
          setLive(false);
          await new Promise((r) => (timer = setTimeout(r, backoff(attempt++))));
        }
      }
    };

    const fallback = (sid: string) => {
      clearTimeout(watch);
      es?.close();
      es = null;
      setLive(false);
      void poll(sid);
    };

    // drop closes src and reopens a stream after the backoff, with a fresh snapshot.
    const drop = (src: EventSourceLike) => {
      src.close();
      if (es !== src || closed) return;
      clearTimeout(watch);
      es = null;
      late = 0;
      setLive(false);
      const delay = backoff(attempt++);
      timer = setTimeout(() => {
        if (closed) return;
        void snapshot();
        open();
      }, delay);
    };

    const open = () => {
      const sid = streamId();
      const src = this.env.eventSource(
        `${room(code)}/events?sid=${encodeURIComponent(sid)}`,
      );
      es = src;
      // the probe runs from here, not from `open`: a proxy that holds even the headers never fires it
      const arm = (ms: number) => {
        clearTimeout(watch);
        const due = Date.now() + ms;
        watch = setTimeout(() => {
          if (es !== src || closed) return;
          // fired late: the page was frozen (a locked phone), not the stream
          if (settling() || Date.now() - due > FROZEN_MS) drop(src);
          else fallback(sid);
        }, ms);
      };
      arm(PROBE_MS);
      src.addEventListener("open", () => {
        attempt = 0;
        setLive(true);
      });
      src.addEventListener("ping", () => {
        if (es === src && !closed) arm(WATCHDOG_MS);
      });
      src.addEventListener("state", (e) => {
        if (closed || es !== src) return;
        attempt = 0;
        arm(WATCHDOG_MS);
        let s: RoomState;
        try {
          s = JSON.parse(e.data as string) as RoomState;
        } catch {
          return; // a garbled event: the next one (or the snapshot after a reconnect) is whole
        }
        if (ref - (s.now - Date.now()) > LAG_MS) {
          apply(s, false); // late: its `now` would set the clock behind
          // states queued while the page slept arrive old: no stall
          if (settling()) late = 0;
          else if (++late >= LAG_EVENTS) fallback(sid);
          return;
        }
        late = 0;
        apply(s);
      });
      src.addEventListener("error", () => drop(src));
    };

    void snapshot();
    open();
    return stop;
  }
}

// The stall detection of subscribeRoom. HEARTBEAT_MS is the server's ping (party/service.go heartbeat).
// FROZEN_MS is how late a timer may fire before the page counts as having slept.
export const PROBE_MS = 5_000;
export const HEARTBEAT_MS = 10_000;
export const WATCHDOG_MS = 2 * HEARTBEAT_MS + 5_000;
export const LAG_MS = 2_000;
export const LAG_EVENTS = 2;
export const RESUME_MS = 2 * PROBE_MS; // a stream reopened on waking still gets its probe in
export const FROZEN_MS = 2_000;

// streamId names a stream for a later poll to end it: random, so another phone behind the same address cannot
// guess it.
export function streamId(): string {
  const b = new Uint8Array(16);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}
