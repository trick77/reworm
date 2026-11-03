import { useEffect, useState, type ReactNode } from "react";
import { cx } from "../lib/cx";
import { Icon, type IconName } from "./Icon";

export type Tab = "home" | "party" | "train" | "settings";

const TABS: { tab: Tab; label: string; icon: IconName }[] = [
  { tab: "home", label: "Home", icon: "home" },
  { tab: "party", label: "Party", icon: "party" },
  { tab: "train", label: "Train", icon: "train" },
  { tab: "settings", label: "Settings", icon: "gear" },
];

// AppShell is one screen: the status-bar safe area, the scrolling content, an optional sticky call to
// action and the bottom tabs (only when `tab` is set: play, reveal and the party screens have none).
export function AppShell({
  tab,
  onTab,
  footer,
  scrollClassName,
  children,
}: {
  tab?: Tab;
  onTab?: (t: Tab) => void;
  footer?: ReactNode;
  scrollClassName?: string;
  children: ReactNode;
}) {
  return (
    <div className="app">
      <div className="status-safe" />
      <div className={cx("scroll", scrollClassName)}>{children}</div>
      {footer && <div className="sticky-cta">{footer}</div>}
      {tab && (
        <nav className="tabs" aria-label="Main">
          {TABS.map((t) => (
            <button
              key={t.tab}
              type="button"
              className={cx(t.tab === tab && "on")}
              aria-current={t.tab === tab ? "page" : undefined}
              onClick={() => onTab?.(t.tab)}
            >
              <Icon name={t.icon} />
              {t.label}
            </button>
          ))}
        </nav>
      )}
    </div>
  );
}

// The stage's design box: the iPad landscape layout, scaled to fill a laptop or a TV across the room.
export const STAGE_W = 1180;
export const STAGE_H = 820;

// stageZoom is the scale of the design box for a landscape screen; null in portrait, where the stage
// fills the screen unscaled (iPad portrait has its own stacked layout).
export function stageZoom(w: number, h: number): number | null {
  if (h > w) return null;
  return Math.min(w / STAGE_W, h / STAGE_H);
}

function windowSize() {
  return { w: window.innerWidth, h: window.innerHeight };
}

export function StageScale({ children }: { children: ReactNode }) {
  const [size, setSize] = useState(windowSize);
  useEffect(() => {
    const on = () => setSize(windowSize());
    window.addEventListener("resize", on);
    return () => window.removeEventListener("resize", on);
  }, []);
  const zoom = stageZoom(size.w, size.h);
  return (
    <div className="stage-fit">
      <div
        className={cx("stage-box", zoom == null && "portrait")}
        style={zoom == null ? undefined : { zoom }}
      >
        {children}
      </div>
    </div>
  );
}
