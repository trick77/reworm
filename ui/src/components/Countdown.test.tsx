import { act, render, screen } from "@testing-library/react";
import { syncServerClock } from "../api/clock";
import {
  CountdownRing,
  StartCountdown,
  TimerBar,
  useSecondsTo,
} from "./Countdown";

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
    expect(vi.getTimerCount()).toBe(0); // the tick stops at 0
  });

  it("drains the ring over the whole window", () => {
    const { container } = render(<CountdownRing seconds={15} size={104} />);
    const bar = container.querySelector<SVGCircleElement>("circle.bar")!;
    expect(bar.style.animationDuration).toBe("15s");
    expect(bar.style.getPropertyValue("--c")).toBe(String(2 * Math.PI * 46));
  });

  it("keeps the seconds in step with the ring across pauses", () => {
    const { rerender } = render(<CountdownRing seconds={15} />);
    act(() => vi.advanceTimersByTime(3500));
    expect(screen.getByRole("timer")).toHaveTextContent("12");
    rerender(<CountdownRing seconds={15} paused />);
    act(() => vi.advanceTimersByTime(5000));
    expect(screen.getByRole("timer")).toHaveTextContent("12");
    rerender(<CountdownRing seconds={15} />);
    // 3.5 s + 0.6 s run: 4.1 s gone, 11 left (a count restarted at resume would still say 12)
    act(() => vi.advanceTimersByTime(600));
    expect(screen.getByRole("timer")).toHaveTextContent("11");
  });

  it("holds while paused", () => {
    const { container } = render(<CountdownRing seconds={15} paused />);
    act(() => vi.advanceTimersByTime(5000));
    expect(screen.getByRole("timer")).toHaveTextContent("15");
    expect(container.firstChild).toHaveClass("paused");
  });
});

describe("StartCountdown", () => {
  afterEach(() => syncServerClock(Date.now()));

  // Counter is what stage and pad do: the seconds by the server clock, the countdown until they run out.
  function Counter({ at }: { at: number }) {
    const left = useSecondsTo(at);
    return left ? <StartCountdown left={left} round={2} /> : <p>Live</p>;
  }

  it("counts 3, 2, 1 to the start by the server clock, on the second, then stops", () => {
    // this device runs 5 s behind the server: the digits follow the server's time
    syncServerClock(Date.now() + 5000);
    render(<Counter at={Date.now() + 5000 + 3000} />);
    const timer = screen.getByRole("timer");
    expect(timer).toHaveTextContent("3");
    expect(timer).toHaveTextContent("Round 2 · get ready");
    expect(timer).toHaveAccessibleName("Round 2 starts in 3");
    act(() => vi.advanceTimersByTime(999));
    expect(timer).toHaveTextContent("3");
    act(() => vi.advanceTimersByTime(1));
    expect(timer).toHaveTextContent("2");
    act(() => vi.advanceTimersByTime(1000));
    expect(timer).toHaveTextContent("1");
    act(() => vi.advanceTimersByTime(1000));
    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(vi.getTimerCount()).toBe(0); // nothing ticks once it ran out
  });
});

describe("TimerBar", () => {
  it("drains over the window and pauses", () => {
    const { container } = render(<TimerBar seconds={15} paused />);
    expect(container.firstChild).toHaveClass("tbar", "paused");
    expect(container.querySelector("i")!.style.animationDuration).toBe("15s");
  });
});
