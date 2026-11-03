import { useState } from "react";
import { useLocation } from "wouter";
import type { ApiError } from "../../api/client";
import { toApiError, useApi } from "../../api/context";
import type { TrainSettings } from "../../api/types";
import { unlockAudio } from "../../audio/unlock";
import { save } from "../../lib/prefs";

// useStartTraining starts a session from a tap (which unlocks the audio) and opens it.
export function useStartTraining() {
  const api = useApi();
  const [, go] = useLocation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const start = (s: TrainSettings) => {
    unlockAudio();
    save("train", s);
    setBusy(true);
    setError(null);
    api.startTraining(s).then(
      (r) => go(`/train/${encodeURIComponent(r.id)}`),
      (e: unknown) => {
        setError(toApiError(e));
        setBusy(false);
      },
    );
  };
  return { start, busy, error };
}
