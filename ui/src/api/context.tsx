import {
  createContext,
  use,
  useEffect,
  useEffectEvent,
  useState,
  type ReactNode,
} from "react";
import { ApiError, HttpApi, type Api } from "./client";
import type { Me } from "./types";

const ApiContext = createContext<Api | null>(null);

let fallback: Api | null = null;

// useApi is the app's API: the provider's, else the real HTTP one.
export function useApi(): Api {
  const api = use(ApiContext);
  if (api) return api;
  fallback ??= new HttpApi();
  return fallback;
}

export const toApiError = (e: unknown): ApiError =>
  e instanceof ApiError
    ? e
    : new ApiError(0, "unknown", e instanceof Error ? e.message : String(e));

type Loaded<T> = { key: string; tick: number; data?: T; error?: ApiError };

// useAsync loads fn() for `key` (null: nothing to load) and again on reload(). Data belongs to its key:
// another key shows none until its own is in; a reload keeps the key's data meanwhile (`loading`).
export function useAsync<T>(key: string | null, fn: () => Promise<T>) {
  const [state, setState] = useState<Loaded<T> | null>(null);
  const [tick, setTick] = useState(0);
  const load = useEffectEvent(fn);
  useEffect(() => {
    if (key == null) return;
    let live = true;
    load().then(
      (data) => live && setState({ key, tick, data }),
      (e: unknown) => live && setState({ key, tick, error: toApiError(e) }),
    );
    return () => {
      live = false;
    };
  }, [key, tick]);
  const mine = key != null && state?.key === key;
  const current = mine && state.tick === tick;
  return {
    data: mine ? state.data : undefined,
    error: current ? state!.error : undefined,
    loading: key != null && !current,
    reload: () => setTick((t) => t + 1),
  };
}

type MeState = {
  me: Me | null;
  // unpaired: the server answered 401, this device has no profile
  unpaired: boolean;
  // error: any other failure (server down): screens that need a profile show it with a retry
  error: ApiError | null;
  loading: boolean;
  reload: () => void;
};

const MeContext = createContext<MeState | null>(null);

function MeLoader({ children }: { children: ReactNode }) {
  const api = useApi();
  const r = useAsync("me", () => api.me());
  const value: MeState = {
    me: r.error ? null : (r.data ?? null),
    unpaired: r.error?.status === 401,
    error: r.error && r.error.status !== 401 ? r.error : null,
    loading: r.loading,
    reload: r.reload,
  };
  return <MeContext value={value}>{children}</MeContext>;
}

export function ApiProvider({
  api,
  children,
}: {
  api: Api;
  children: ReactNode;
}) {
  return (
    <ApiContext value={api}>
      <MeLoader>{children}</MeLoader>
    </ApiContext>
  );
}

// useMe is the paired profile of this device, loaded once per app start (reload() after a change).
export function useMe(): MeState {
  const m = use(MeContext);
  if (!m) throw new Error("useMe outside ApiProvider");
  return m;
}
