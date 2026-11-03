import { cx } from "../lib/cx";
import { Icon } from "./Icon";

export type Choice = "earlier" | "exact" | "later";

const BUTTONS = [
  { choice: "earlier", label: "Earlier", icon: "left", key: "←" },
  { choice: "exact", label: "Spot on", icon: "target", key: "space" },
  { choice: "later", label: "Later", icon: "right", key: "→" },
] as const;

// AnswerPad is the three answers. "choices": the training row under the year card, with keyboard hints;
// "pad": the party phone's big stacked buttons.
export function AnswerPad({
  onAnswer,
  variant = "choices",
  disabled,
  keys = true,
}: {
  onAnswer: (c: Choice) => void;
  variant?: "choices" | "pad";
  disabled?: boolean;
  keys?: boolean;
}) {
  if (variant === "pad") {
    return (
      <div className="padbtns">
        {BUTTONS.map((b) => (
          <button
            key={b.choice}
            type="button"
            className={cx("padbtn", b.choice)}
            disabled={disabled}
            onClick={() => onAnswer(b.choice)}
          >
            <Icon name={b.icon} />
            {b.label}
          </button>
        ))}
      </div>
    );
  }
  return (
    // data-answer: the year card's keys still work with one of these focused (after a click)
    <div className="choices" data-answer="">
      {BUTTONS.map((b) => (
        <button
          key={b.choice}
          type="button"
          className={cx("choice", b.choice === "exact" && "mid")}
          disabled={disabled}
          onClick={() => onAnswer(b.choice)}
        >
          <Icon name={b.icon} />
          {b.label}
          {keys && (
            <small aria-hidden="true">
              <span className="kbd">{b.key}</span>
            </small>
          )}
        </button>
      ))}
    </div>
  );
}
