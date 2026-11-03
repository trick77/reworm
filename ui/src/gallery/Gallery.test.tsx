import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { fakeEnv } from "../audio/fakes";
import Gallery from "./Gallery";

// The gallery's audio demo plays through the shared player; give it fakes instead of jsdom's media.
const fake = fakeEnv();
vi.mock("../audio/player", async (orig) => {
  const mod = await orig<typeof import("../audio/player")>();
  let p: InstanceType<typeof mod.PreviewPlayer> | null = null;
  return {
    ...mod,
    player: () =>
      (p ??= new mod.PreviewPlayer({ engine: "webaudio", env: fake.env })),
  };
});

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe("Gallery", () => {
  it("renders every component section and the party screens", () => {
    render(<Gallery />);
    for (const t of [
      "Buttons",
      "Avatars · 12",
      "Year card · answers",
      "Reveal · bullseye",
      "Party stage (iPad / laptop / TV)",
      "Party pad (phone)",
    ])
      expect(screen.getByText(t)).toBeInTheDocument();
    for (const c of [
      "Stage · round live",
      "Stage · round result",
      "Stage · game over",
      "Pad · round live",
      "Pad · locked out",
      "Pad · you got it",
      "Pad · someone was faster",
    ])
      expect(screen.getByText(c)).toBeInTheDocument();
    expect(screen.getByText("Mia wins!")).toBeInTheDocument();
    expect(screen.getByText("Sarah was faster")).toBeInTheDocument();
  });

  it("drives the demos: tabs, chips, avatar, answers, timers", () => {
    render(<Gallery />);
    fireEvent.click(screen.getByRole("button", { name: "Party" }));
    expect(screen.getByRole("button", { name: "Party" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    const mega = screen.getByRole("button", { name: /Mega Mix/ });
    fireEvent.click(mega);
    expect(mega).toHaveClass("on");
    fireEvent.click(mega);
    expect(mega).not.toHaveClass("on");
    fireEvent.click(screen.getByRole("button", { name: "Hard" }));
    expect(screen.getByRole("button", { name: "Hard" })).toHaveClass("on");
    fireEvent.click(screen.getByRole("radio", { name: "Woofer" }));
    expect(screen.getByRole("radio", { name: "Woofer" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    fireEvent.keyDown(window, { key: "ArrowRight" });
    act(() => vi.advanceTimersByTime(300));
    expect(screen.getByText("Answered: later")).toBeInTheDocument();
    const timers = within(
      screen.getByRole("button", { name: "Restart" }).closest(".card")!,
    );
    fireEvent.click(timers.getByRole("button", { name: "Pause" }));
    expect(timers.getAllByRole("timer")[0]).toHaveClass("paused");
    fireEvent.click(timers.getByRole("button", { name: "Resume" }));
    fireEvent.click(timers.getByRole("button", { name: "Restart" }));
    expect(timers.getAllByRole("timer")[0]).toHaveTextContent("15");
  });

  it("opens the stage full screen, scaled, and closes it", () => {
    const { container } = render(<Gallery />);
    fireEvent.click(screen.getByRole("button", { name: /Stage full screen/ }));
    expect(container.querySelector(".stage-fit .stage-box")).not.toBeNull();
    expect(screen.queryByText("Buttons")).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Close full-screen stage" }),
    );
    expect(screen.getByText("Buttons")).toBeInTheDocument();
  });

  it("plays a preview, pauses, switches engine", async () => {
    render(<Gallery />);
    const card = screen.getByText("Next").closest(".card") as HTMLElement;
    const ctl = within(card);
    fireEvent.click(ctl.getByRole("button", { name: "Play" }));
    const el = fake.audios.at(-1)!;
    expect(el.src).toContain("audio-ssl.itunes.apple.com");
    expect(ctl.getByText(/Like a Prayer/)).toBeInTheDocument();
    fireEvent.click(ctl.getByRole("button", { name: "Pause" }));
    await act(() => vi.advanceTimersByTimeAsync(400));
    fireEvent.click(ctl.getByRole("button", { name: "Play" }));
    fireEvent.click(ctl.getByRole("button", { name: "Next" }));
    expect(ctl.getByText(/Dancing Queen/)).toBeInTheDocument();
    fireEvent.click(ctl.getByRole("button", { name: "Replay" }));
    fireEvent.click(ctl.getByRole("button", { name: "<audio>" }));
    expect(ctl.getByRole("button", { name: "<audio>" })).toHaveClass("on");
    expect(window.localStorage.getItem("reworm.audioEngine")).toBe("element");
  });
});
