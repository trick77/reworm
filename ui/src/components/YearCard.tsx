import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  type PointerEvent,
  type ReactNode,
} from "react";
import { cx } from "../lib/cx";
import { AnswerPad, type Choice } from "./AnswerPad";

// A swipe past SWIPE px answers; the card flies off for FLY ms before onAnswer fires (the mock's flashCard).
export const SWIPE = 90;
export const FLY = 180;

const TINT: Record<Choice, string> = {
  earlier: "← EARLIER",
  exact: "↑ SPOT ON",
  later: "LATER →",
};

type Drag = { dx: number; dy: number };

// Keys on a focused control belong to it (Space presses a button, arrows move through a radio group or a
// slider); the card's own answer buttons ([data-answer]) are the exception.
const INTERACTIVE = [
  "button",
  "a[href]",
  "input",
  "select",
  "textarea",
  "summary",
  "[contenteditable]",
  "[tabindex]:not([tabindex='-1'])",
  ...[
    "button",
    "radio",
    "tab",
    "checkbox",
    "switch",
    "slider",
    "spinbutton",
    "option",
    "menuitem",
    "link",
    "textbox",
  ].map((r) => `[role=${r}]`),
].join(",");

function ownsKeys(t: EventTarget | null): boolean {
  if (!(t instanceof Element)) return false;
  if (t.closest("[data-answer]")) return false;
  if (t instanceof HTMLElement && t.isContentEditable) return true;
  return t.closest(INTERACTIVE) != null;
}

// YearCard is the big proposed year. With onAnswer it is the answer: swipe left (earlier), up (spot on) or
// right (later), keys ← space/↑ → when `keyboard` is on (one card per screen), and the button row when
// `choices` is on. Without onAnswer it is display only (the stage). Key it per card: a flown card stays gone.
export function YearCard({
  year,
  question = "Released …",
  hint = "swipe ← earlier · ↑ spot on · later →",
  onAnswer,
  keyboard,
  choices,
  className,
}: {
  year: ReactNode;
  question?: ReactNode;
  hint?: ReactNode;
  onAnswer?: (c: Choice) => void;
  keyboard?: boolean;
  choices?: boolean;
  className?: string;
}) {
  const [drag, setDrag] = useState<Drag | null>(null);
  const [flying, setFlying] = useState<Choice | null>(null);
  const start = useRef<{ x: number; y: number } | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const live = !!onAnswer && !flying;

  // answered: set at once, unlike the flying state, so two answers in one frame (a key and a tap) submit once
  const answered = useRef(false);

  useEffect(() => () => clearTimeout(timer.current), []);

  const fly = (c: Choice) => {
    if (!onAnswer || answered.current) return;
    answered.current = true;
    setFlying(c);
    timer.current = setTimeout(() => onAnswer(c), FLY);
  };

  const onKey = useEffectEvent((e: KeyboardEvent) => {
    if (ownsKeys(e.target) || e.metaKey || e.ctrlKey || e.altKey) return;
    const c: Choice | null =
      e.key === "ArrowLeft"
        ? "earlier"
        : e.key === "ArrowRight"
          ? "later"
          : e.key === " " || e.key === "ArrowUp"
            ? "exact"
            : null;
    if (!c) return;
    e.preventDefault();
    fly(c);
  });

  useEffect(() => {
    if (!keyboard) return;
    const h = (e: KeyboardEvent) => onKey(e);
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [keyboard]);

  const down = (e: PointerEvent<HTMLDivElement>) => {
    if (!live) return;
    start.current = { x: e.clientX, y: e.clientY };
    setDrag({ dx: 0, dy: 0 });
    e.currentTarget.setPointerCapture?.(e.pointerId);
  };
  const move = (e: PointerEvent<HTMLDivElement>) => {
    const s = start.current;
    if (!s) return;
    setDrag({ dx: e.clientX - s.x, dy: Math.min(0, e.clientY - s.y) });
  };
  const up = (e: PointerEvent<HTMLDivElement>) => {
    const s = start.current;
    if (!s) return;
    start.current = null;
    setDrag(null);
    const dx = e.clientX - s.x;
    const dy = e.clientY - s.y;
    if (dx < -SWIPE) fly("earlier");
    else if (dx > SWIPE) fly("later");
    else if (dy < -SWIPE) fly("exact");
  };
  const cancel = () => {
    start.current = null;
    setDrag(null);
  };

  let transform: string | undefined;
  let tint: Choice | null = null;
  let mag = 0;
  if (flying) {
    const dx = flying === "earlier" ? -420 : flying === "later" ? 420 : 0;
    const dy = flying === "exact" ? -260 : 0;
    transform = `translate(${dx}px,${dy}px) rotate(${dx / 30}deg)`;
    tint = flying;
    mag = 1;
  } else if (drag) {
    const { dx, dy } = drag;
    transform = `translate(${dx}px,${dy}px) rotate(${dx / 22}deg)`;
    mag = Math.min(1, Math.max(Math.abs(dx), -dy) / 110);
    if (mag > 0)
      tint = -dy > Math.abs(dx) ? "exact" : dx < 0 ? "earlier" : "later";
  }

  return (
    <>
      <div
        className={cx(
          "yearcard",
          drag && "drag",
          flying && "flying",
          !onAnswer && "static",
          className,
        )}
        style={{ transform }}
        data-testid="yearcard"
        onPointerDown={down}
        onPointerMove={move}
        onPointerUp={up}
        onPointerCancel={cancel}
      >
        <div className={cx("tint", tint)} style={{ opacity: mag }}>
          {tint && <span>{TINT[tint]}</span>}
        </div>
        <div className="q" style={{ opacity: 1 - mag }}>
          {question}
        </div>
        <div className="y">{year}</div>
        {hint && <div className="hint">{hint}</div>}
      </div>
      {choices && onAnswer && (
        <AnswerPad onAnswer={fly} disabled={!!flying} keys={!!keyboard} />
      )}
    </>
  );
}
