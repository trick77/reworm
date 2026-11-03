import { cx } from "../lib/cx";

// Vinyl is the spinning record that stands in for the song while its title is still secret.
export function Vinyl({
  spinning,
  label = "?",
}: {
  spinning?: boolean;
  label?: string;
}) {
  return (
    <div className="vinyl-wrap">
      <div className={cx("vinyl", spinning && "spin")} data-testid="vinyl">
        <div className="lbl">
          <span>{label}</span>
        </div>
      </div>
    </div>
  );
}
