import { useEffect, useState } from "react";
import type { ApiError } from "./client";
import { useApi } from "./context";
import type { RoomState } from "./types";

type Feed = {
  code: string | null;
  state?: RoomState;
  error?: ApiError;
  live: boolean;
};

// useRoom follows a room's live state (snapshot + event stream, reconnecting by itself).
export function useRoom(code: string | null) {
  const api = useApi();
  const [feed, setFeed] = useState<Feed>({ code: null, live: false });
  useEffect(() => {
    if (!code) return;
    const mine = (p: Feed): Feed =>
      p.code === code ? p : { code, live: false };
    return api.subscribeRoom(code, {
      onState: (state) =>
        setFeed((p) => ({ ...mine(p), state, error: undefined })),
      onError: (error) => setFeed((p) => ({ ...mine(p), error })),
      onLive: (live) => setFeed((p) => ({ ...mine(p), live })),
    });
  }, [api, code]);
  const mine = feed.code === code;
  return {
    state: mine ? feed.state : undefined,
    error: mine ? feed.error : undefined,
    live: mine && feed.live,
  };
}
