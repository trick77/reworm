import { render, screen } from "@testing-library/react";
import { Ruler, Verdict, rulerLayout } from "./Verdict";

describe("rulerLayout", () => {
  it("spans 4 years past both, a big tick every 5", () => {
    const r = rulerLayout(1980, 1976);
    expect(r.ticks[0]).toMatchObject({ year: 1972, at: 0, big: false });
    expect(r.ticks.at(-1)).toMatchObject({ year: 1984, at: 100 });
    expect(r.ticks.filter((t) => t.big).map((t) => t.year)).toEqual([
      1975, 1980,
    ]);
    expect(r.gap!.left).toBeCloseTo((4 / 12) * 100);
    expect(r.gap!.width).toBeCloseTo((4 / 12) * 100);
  });

  it("grows the earlier label left and the later one right", () => {
    expect(rulerLayout(1988, 1989).markers).toEqual([
      expect.objectContaining({ kind: "shown", year: 1988, grow: "left" }),
      expect.objectContaining({ kind: "actual", year: 1989, grow: "right" }),
    ]);
    expect(rulerLayout(1990, 1989).markers).toEqual([
      expect.objectContaining({ kind: "shown", grow: "right" }),
      expect.objectContaining({ kind: "actual", grow: "left" }),
    ]);
  });

  it("draws one centered marker and no gap for the same year", () => {
    const r = rulerLayout(1975, 1975);
    expect(r.gap).toBeNull();
    expect(r.markers).toEqual([
      expect.objectContaining({ kind: "actual", grow: "center", at: 50 }),
    ]);
  });
});

describe("Ruler", () => {
  it("labels shown and actual, growing apart", () => {
    render(<Ruler shown={1988} actual={1989} />);
    expect(screen.getByText("shown 1988")).toHaveClass("mk", "grow-left");
    expect(screen.getByText("actual 1989")).toHaveClass("mk", "grow-right");
    expect(screen.getByText("1985")).toHaveClass("tl");
  });

  it("says `you` in pin mode", () => {
    render(<Ruler shown={1990} actual={1991} mode="pin" />);
    expect(screen.getByText("you 1990")).toBeInTheDocument();
  });
});

describe("Verdict", () => {
  it("stamps a hit and shows the year, tier, why and ruler", () => {
    const { container } = render(
      <Verdict
        result="hit"
        year={1976}
        what="Yes, earlier"
        why="+20 XP"
        tier={{ label: "Clear one", color: "var(--jade)" }}
        ruler={{ shown: 1980 }}
      />,
    );
    expect(container.firstChild).toHaveClass("verdict", "ok");
    expect(screen.getByText("HIT")).toHaveClass("stamp");
    expect(screen.getByText("1976")).toHaveClass("big");
    expect(screen.getByText("Clear one")).toBeInTheDocument();
    expect(screen.getByText("+20 XP")).toBeInTheDocument();
    expect(screen.getByTestId("ruler")).toBeInTheDocument();
  });

  it("stamps a miss red and a bullseye in two lines", () => {
    const { container, rerender } = render(
      <Verdict result="miss" year={1989} what="It's later" />,
    );
    expect(container.firstChild).toHaveClass("bad");
    expect(screen.getByText("MISS")).toBeInTheDocument();
    expect(screen.queryByTestId("ruler")).toBeNull();
    rerender(<Verdict result="bullseye" year={1975} what="Spot on" />);
    expect(container.querySelector(".stamp")!.innerHTML).toBe("BULLS<br>EYE");
  });

  it("leaves the stamp out on the stage", () => {
    const { container } = render(
      <Verdict result="hit" year={1989} what="Later" stamp={false} center />,
    );
    expect(container.querySelector(".stamp")).toBeNull();
    expect(container.firstChild).toHaveClass("center");
  });
});
