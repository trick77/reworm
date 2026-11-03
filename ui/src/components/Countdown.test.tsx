import { act, render, screen } from "@testing-library/react";
import { CountdownRing, TimerBar } from "./Countdown";

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("CountdownRing", () => {
  it("counts the seconds down and fires onDone at 0", () => {
    const onDone = vi.fn();
    render(<CountdownRing seconds={15} onDone={onDone} />);
    expect(screen.getByRole("timer")).toHaveTextContent("15");
    act(() => vi.advanceTimersByTime(3000));
    expect(screen.getByRole("timer")).toHaveTextContent("12");
    expect(screen.getByRole("timer")).toHaveAccessibleName("12 seconds left");
    act(() => vi.advanceTimersByTime(12000));
    expect(screen.getByRole("timer")).toHaveTextContent("0");
    expect(onDone).toHaveBeenCalledTimes(1);
    act(() => vi.advanceTimersByTime(5000));
    expect(screen.getByRole("timer")).toHaveTextContent("0");
    expect(onDone).toHaveBeenCalledTimes(1);
  });

  it("drains the ring over the whole window", () => {
    const { container } = render(<CountdownRing seconds={15} size={104} />);
    const bar = container.querySelector<SVGCircleElement>("circle.bar")!;
    expect(bar.style.animationDuration).toBe("15s");
    expect(bar.style.getPropertyValue("--c")).toBe(String(2 * Math.PI * 46));
  });

  it("holds while paused", () => {
    const { container } = render(<CountdownRing seconds={15} paused />);
    act(() => vi.advanceTimersByTime(5000));
    expect(screen.getByRole("timer")).toHaveTextContent("15");
    expect(container.firstChild).toHaveClass("paused");
  });
});

describe("TimerBar", () => {
  it("drains over the window and pauses", () => {
    const { container } = render(<TimerBar seconds={15} paused />);
    expect(container.firstChild).toHaveClass("tbar", "paused");
    expect(container.querySelector("i")!.style.animationDuration).toBe("15s");
  });
});
