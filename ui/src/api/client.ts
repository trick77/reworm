// The API client: every endpoint of the contract behind one interface, so screens and tests can swap the
// network for a fake. HttpApi is the real one: fetch for the calls, EventSource for the live room state.
import { syncServerClock } from "./clock";
import type {
  Choice,
  Decks,
  Device,
  Me,
  PairCode,
  PairKind,
  Points,
  Profile,
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

export type HostAction = "start" | "next" | "pause" | "resume" | "rematch";

export type RoomHandlers = {
  onState: (s: RoomState) => void;
  // onError: the snapshot could not be fetched. A 404 (the room is gone) ends the subscription.
  onError?: (e: ApiError) => void;
  // onLive: the event stream is connected (true) or dropped and reconnecting (false)
  onLive?: (live: boolean) => void;
};

export interface Api {
  version(): Promise<string>;
  me(): Promise<Me>;
  setup(body: { token: string; name: string; avatar: string }): Promise<void>;
  profiles(): Promise<Profile[]>;
  switchProfile(profileId: string): Promise<void>;
  updateMe(body: { name?: string; avatar?: string }): Promise<void>;
  devices(): Promise<Device[]>;
  removeDevice(id: string): Promise<void>;
  pair(kind: PairKind): Promise<PairCode>;
  claimPair(body: {
    code: string;
    name?: string;
    avatar?: string;
  }): Promise<void>;
  decks(): Promise<Decks>;
  coverUrl(songId: string): string;

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
  host(code: string, action: HostAction): Promise<void>;
  kick(code: string, playerId: string): Promise<void>;
  roomSettings(code: string, settings: RoomSettings): Promise<void>;
  answer(code: string, round: number, choice: Choice): Promise<void>;
  buzz(code: string, round: number): Promise<void>;
  reveal(code: string, round: number): Promise<void>;
  judge(code: string, round: number, points: Points): Promise<void>;

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

function browserEnv(): HttpEnv {
  return {
    fetch: (input, init) => window.fetch(input, init),
    eventSource: (url) => new EventSource(url, { withCredentials: true }),
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
  profiles = () => this.get<Profile[]>("/api/profiles");
  switchProfile = (profileId: string) =>
    this.post("/api/profiles/switch", { profileId });
  updateMe = (b: { name?: string; avatar?: string }) =>
    this.call<void>("PATCH", "/api/me", b);
  devices = () => this.get<Device[]>("/api/devices");
  removeDevice = (id: string) =>
    this.call<void>("DELETE", `/api/devices/${encodeURIComponent(id)}`);
  pair = (kind: PairKind) => this.post<PairCode>("/api/pair", { kind });
  claimPair = (b: { code: string; name?: string; avatar?: string }) =>
    this.post("/api/pair/claim", b);
  decks = () => this.get<Decks>("/api/decks");
  coverUrl = (songId: string) => `/api/covers/${encodeURIComponent(songId)}`;

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
  host = (code: string, action: HostAction) =>
    this.post(`${room(code)}/${action}`);
  kick = (code: string, playerId: string) =>
    this.post(`${room(code)}/kick`, { playerId });
  roomSettings = (code: string, s: RoomSettings) =>
    this.post(`${room(code)}/settings`, s);
  answer = async (code: string, round: number, choice: Choice) => {
    await this.post(`${room(code)}/answer`, { round, choice });
  };
  buzz = (code: string, round: number) =>
    this.post(`${room(code)}/buzz`, { round });
  reveal = (code: string, round: number) =>
    this.post(`${room(code)}/reveal`, { round });
  judge = (code: string, round: number, points: Points) =>
    this.post(`${room(code)}/judge`, { round, points });

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

  // subscribeRoom: snapshot first, then the event stream. A dropped stream is closed (EventSource's own
  // retry would skip what changed meanwhile) and reopened after a backoff, with a fresh snapshot.
  subscribeRoom(code: string, h: RoomHandlers): () => void {
    let closed = false;
    let es: EventSourceLike | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    // order: a state older than the last applied one (by seq) is dropped, snapshot or event. One of the same
    // seq changes nothing but still sets the server clock (`now`).
    let lastSeq = -Infinity;
    const apply = (s: RoomState) => {
      if (s.seq < lastSeq) return;
      syncServerClock(s.now);
      if (s.seq === lastSeq) return;
      lastSeq = s.seq;
      h.onState(s);
    };
    const stop = () => {
      closed = true;
      clearTimeout(timer);
      es?.close();
      es = null;
    };

    const snapshot = () =>
      this.room(code).then(
        (s) => {
          if (!closed) apply(s);
        },
        (e: ApiError) => {
          if (closed) return;
          if (e.status === 404) stop(); // the room is gone: nothing to reconnect to
          h.onError?.(e);
        },
      );

    const open = () => {
      const src = this.env.eventSource(`${room(code)}/events`);
      es = src;
      src.addEventListener("open", () => {
        attempt = 0;
        h.onLive?.(true);
      });
      src.addEventListener("state", (e) => {
        if (closed) return;
        attempt = 0;
        try {
          apply(JSON.parse(e.data as string) as RoomState);
        } catch {
          // a garbled event: the next one (or the snapshot after a reconnect) is whole
        }
      });
      src.addEventListener("error", () => {
        src.close();
        if (es !== src || closed) return;
        es = null;
        h.onLive?.(false);
        const delay = backoff(attempt++);
        timer = setTimeout(() => {
          if (closed) return;
          void snapshot();
          open();
        }, delay);
      });
    };

    void snapshot();
    open();
    return stop;
  }
}
