import { act, render, renderHook, screen } from "@testing-library/react";
import { serverNow, syncServerClock } from "../api/clock";
import { useAsync } from "../api/context";
import { DeadlineBar, DeadlineRing } from "./Countdown";

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  vi.useRealTimers();
  syncServerClock(Date.now());
});

describe("DeadlineRing", () => {
  it("counts to the server deadline; a re-render (a new room state) neither restarts nor speeds it up", () => {
    const endsAt = serverNow() + 15_000;
    const { rerender } = render(<DeadlineRing endsAt={endsAt} total={15} />);
    expect(screen.getByRole("timer")).toHaveTextContent("15");
    act(() => vi.advanceTimersByTime(5_000));
    expect(screen.getByRole("timer")).toHaveTextContent("10");
    rerender(<DeadlineRing endsAt={endsAt} total={15} />);
    act(() => vi.advanceTimersByTime(250));
    expect(screen.getByRole("timer")).toHaveTextContent("10");
    act(() => vi.advanceTimersByTime(1_000));
    expect(screen.getByRole("timer")).toHaveTextContent("9");
  });

  it("holds while paused and goes on from a resume's later deadline", () => {
    const endsAt = serverNow() + 10_000;
    const { rerender } = render(
      <DeadlineRing endsAt={endsAt} total={10} paused />,
    );
    act(() => vi.advanceTimersByTime(4_000));
    expect(screen.getByRole("timer")).toHaveTextContent("10");
    rerender(<DeadlineRing endsAt={endsAt + 4_000} total={10} />);
    act(() => vi.advanceTimersByTime(250));
    expect(screen.getByRole("timer")).toHaveTextContent("10");
  });

  it("shows what is left on a late screen, by the server's clock", () => {
    syncServerClock(Date.now() + 3_000); // the server is 3 s ahead
    render(<DeadlineRing endsAt={serverNow() + 6_000} total={15} />);
    expect(screen.getByRole("timer")).toHaveTextContent("6");
  });
});

describe("DeadlineBar", () => {
  it("drains by the deadline, not by its render count", () => {
    const endsAt = serverNow() + 10_000;
    const { rerender } = render(<DeadlineBar endsAt={endsAt} total={10} />);
    const bar = screen.getByRole("progressbar");
    expect(bar).toHaveAttribute("aria-valuenow", "100");
    act(() => vi.advanceTimersByTime(5_000));
    rerender(<DeadlineBar endsAt={endsAt} total={10} />);
    expect(bar).toHaveAttribute("aria-valuenow", "50");
    act(() => vi.advanceTimersByTime(6_000));
    expect(bar).toHaveAttribute("aria-valuenow", "0");
  });
});

describe("useAsync", () => {
  it("hands out data only for its own key, and keeps it across a reload", async () => {
    const { result, rerender } = renderHook(
      ({ k }: { k: string }) => useAsync(k, () => Promise.resolve(`data:${k}`)),
      { initialProps: { k: "a" } },
    );
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(result.current.data).toBe("data:a");
    rerender({ k: "b" });
    expect(result.current.data).toBeUndefined();
    expect(result.current.loading).toBe(true);
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(result.current.data).toBe("data:b");
    act(() => result.current.reload());
    expect(result.current.data).toBe("data:b");
    expect(result.current.loading).toBe(true);
  });
});
