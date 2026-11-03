import { act, fireEvent, render, screen } from "@testing-library/react";
import { FLY, YearCard } from "./YearCard";

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

function card() {
  return screen.getByTestId("yearcard");
}

function swipe(dx: number, dy: number, steps = true) {
  fireEvent.pointerDown(card(), { clientX: 200, clientY: 300, pointerId: 1 });
  if (steps)
    fireEvent.pointerMove(card(), {
      clientX: 200 + dx / 2,
      clientY: 300 + dy / 2,
      pointerId: 1,
    });
  fireEvent.pointerUp(card(), {
    clientX: 200 + dx,
    clientY: 300 + dy,
    pointerId: 1,
  });
}

describe("YearCard", () => {
  it("shows the year and the question", () => {
    render(<YearCard year={1983} />);
    expect(screen.getByText("1983")).toBeInTheDocument();
    expect(screen.getByText("Released …")).toBeInTheDocument();
    expect(card()).toHaveClass("static");
  });

  it.each([
    [-140, 0, "earlier"],
    [140, 0, "later"],
    [10, -140, "exact"],
  ])("a swipe of (%i, %i) answers %s after the fly-off", (dx, dy, want) => {
    const onAnswer = vi.fn();
    render(<YearCard year={1983} onAnswer={onAnswer} />);
    swipe(dx, dy);
    expect(card()).toHaveClass("flying");
    expect(onAnswer).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(FLY));
    expect(onAnswer).toHaveBeenCalledWith(want);
  });

  it("a short drag snaps back without an answer", () => {
    const onAnswer = vi.fn();
    render(<YearCard year={1983} onAnswer={onAnswer} />);
    swipe(60, -40);
    act(() => vi.advanceTimersByTime(1000));
    expect(onAnswer).not.toHaveBeenCalled();
    expect(card()).not.toHaveClass("flying");
    expect(card().style.transform).toBe("");
  });

  it("tints and labels the card while dragging", () => {
    render(<YearCard year={1983} onAnswer={() => {}} />);
    fireEvent.pointerDown(card(), { clientX: 200, clientY: 300 });
    fireEvent.pointerMove(card(), { clientX: 120, clientY: 300 });
    expect(card()).toHaveClass("drag");
    expect(screen.getByText("← EARLIER")).toBeInTheDocument();
    fireEvent.pointerMove(card(), { clientX: 200, clientY: 180 });
    expect(screen.getByText("↑ SPOT ON")).toBeInTheDocument();
    fireEvent.pointerMove(card(), { clientX: 300, clientY: 300 });
    expect(screen.getByText("LATER →")).toBeInTheDocument();
    fireEvent.pointerCancel(card());
    expect(card()).not.toHaveClass("drag");
  });

  it("ignores pointer moves without a press and drags on a display card", () => {
    const onAnswer = vi.fn();
    render(<YearCard year={1983} />);
    fireEvent.pointerMove(card(), { clientX: 10, clientY: 10 });
    fireEvent.pointerUp(card(), { clientX: 10, clientY: 10 });
    swipe(-200, 0);
    act(() => vi.advanceTimersByTime(1000));
    expect(card()).not.toHaveClass("flying");
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it.each([
    ["ArrowLeft", "earlier"],
    ["ArrowRight", "later"],
    [" ", "exact"],
    ["ArrowUp", "exact"],
  ])("key %j answers %s", (key, want) => {
    const onAnswer = vi.fn();
    render(<YearCard year={1983} onAnswer={onAnswer} keyboard />);
    fireEvent.keyDown(window, { key });
    act(() => vi.advanceTimersByTime(FLY));
    expect(onAnswer).toHaveBeenCalledWith(want);
  });

  it("answers once, ignores other keys, typing and modifiers", () => {
    const onAnswer = vi.fn();
    render(
      <>
        <input aria-label="name" />
        <YearCard year={1983} onAnswer={onAnswer} keyboard />
      </>,
    );
    fireEvent.keyDown(window, { key: "x" });
    fireEvent.keyDown(screen.getByLabelText("name"), { key: "ArrowLeft" });
    fireEvent.keyDown(window, { key: "ArrowLeft", metaKey: true });
    act(() => vi.advanceTimersByTime(1000));
    expect(onAnswer).not.toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    fireEvent.keyDown(window, { key: "ArrowRight" });
    act(() => vi.advanceTimersByTime(1000));
    expect(onAnswer).toHaveBeenCalledTimes(1);
    expect(onAnswer).toHaveBeenCalledWith("earlier");
  });

  it("leaves keys on focused controls to them", () => {
    const onAnswer = vi.fn();
    render(
      <>
        <button>other</button>
        <div role="radiogroup">
          <span role="radio" aria-checked="false" tabIndex={0}>
            Jukey
          </span>
        </div>
        <div role="tablist">
          <span role="tab" tabIndex={0}>
            Train
          </span>
        </div>
        <a href="#x">link</a>
        <div contentEditable suppressContentEditableWarning>
          note
        </div>
        <input type="range" aria-label="year" />
        <YearCard year={1983} onAnswer={onAnswer} keyboard choices />
      </>,
    );
    for (const t of [
      screen.getByText("other"),
      screen.getByRole("radio"),
      screen.getByRole("tab"),
      screen.getByText("link"),
      screen.getByText("note"),
      screen.getByLabelText("year"),
    ]) {
      fireEvent.keyDown(t, { key: " " });
      fireEvent.keyDown(t, { key: "ArrowRight" });
    }
    act(() => vi.advanceTimersByTime(1000));
    expect(onAnswer).not.toHaveBeenCalled();
    // the card's own answer buttons keep the keys (focused after a click)
    fireEvent.keyDown(screen.getByRole("button", { name: /Earlier/ }), {
      key: "ArrowRight",
    });
    act(() => vi.advanceTimersByTime(FLY));
    expect(onAnswer).toHaveBeenCalledWith("later");
  });

  it("has no keys without `keyboard`", () => {
    const onAnswer = vi.fn();
    render(<YearCard year={1983} onAnswer={onAnswer} />);
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    act(() => vi.advanceTimersByTime(1000));
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("answers from the button row, which locks while the card flies", () => {
    const onAnswer = vi.fn();
    render(<YearCard year={1983} onAnswer={onAnswer} choices keyboard />);
    fireEvent.click(screen.getByRole("button", { name: /Spot on/ }));
    expect(screen.getByRole("button", { name: /Later/ })).toBeDisabled();
    act(() => vi.advanceTimersByTime(FLY));
    expect(onAnswer).toHaveBeenCalledWith("exact");
  });

  it("drops a pending answer on unmount", () => {
    const onAnswer = vi.fn();
    const { unmount } = render(<YearCard year={1983} onAnswer={onAnswer} />);
    swipe(-200, 0);
    unmount();
    act(() => vi.advanceTimersByTime(1000));
    expect(onAnswer).not.toHaveBeenCalled();
  });
});
