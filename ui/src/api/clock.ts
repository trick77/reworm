// The server's clock: room times (endsAt, answerEndsAt, judgeEndsAt) are server milliseconds, and a
// phone's clock can be seconds off. A state carrying `now` sets the offset at receipt.

let offset = 0;

export function syncServerClock(
  serverNow: number | undefined,
  local = Date.now(),
) {
  if (serverNow != null) offset = serverNow - local;
}

export const serverNow = () => Date.now() + offset;

// secondsLeft until a server time, whole seconds, never below 0.
export const secondsLeft = (at: number, now = serverNow()) =>
  Math.max(0, Math.ceil((at - now) / 1000));
