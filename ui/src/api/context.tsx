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

type Loaded<T> = { key: string; data?: T; error?: ApiError };

// useAsync loads fn() for `key` (null: nothing to load) and again on reload(). The previous key's data
// stays until the new one is in; `loading` is true meanwhile.
export function useAsync<T>(key: string | null, fn: () => Promise<T>) {
  const [state, setState] = useState<Loaded<T>>({ key: "" });
  const [tick, setTick] = useState(0);
  const load = useEffectEvent(fn);
  useEffect(() => {
    if (key == null) return;
    let live = true;
    const k = `${key}#${tick}`;
    load().then(
      (data) => live && setState({ key: k, data }),
      (e: unknown) => live && setState({ key: k, error: toApiError(e) }),
    );
    return () => {
      live = false;
    };
  }, [key, tick]);
  const current = state.key === `${key}#${tick}`;
  return {
    data: state.data,
    error: current ? state.error : undefined,
    loading: key != null && !current,
    reload: () => setTick((t) => t + 1),
    set: (data: T) => setState({ key: `${key}#${tick}`, data }),
  };
}

type MeState = {
  me: Me | null;
  // unpaired: the server answered 401, this device has no profile
  unpaired: boolean;
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
