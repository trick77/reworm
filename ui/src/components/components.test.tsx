// Render and interaction tests of the smaller components.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { AnswerPad } from "./AnswerPad";
import { AppShell, StageScale, stageZoom } from "./AppShell";
import { Avatar, AvatarPicker } from "./Avatar";
import { AWARD_TIERS, AwardBadge } from "./AwardBadge";
import { Button, Card, Chip, IconButton, Pill, Segmented } from "./Controls";
import { Cover, SongHead } from "./Cover";
import {
  Confetti,
  PlayerStrip,
  Podium,
  Standings,
  type Player,
} from "./Players";
import { Vinyl } from "./Vinyl";
import { ClipTime, Waveform, clock } from "./Waveform";

describe("AnswerPad", () => {
  it.each(["choices", "pad"] as const)(
    "%s calls back with the choice",
    (variant) => {
      const onAnswer = vi.fn();
      render(<AnswerPad variant={variant} onAnswer={onAnswer} />);
      fireEvent.click(screen.getByRole("button", { name: /Earlier/ }));
      fireEvent.click(screen.getByRole("button", { name: /Spot on/ }));
      fireEvent.click(screen.getByRole("button", { name: /Later/ }));
      expect(onAnswer.mock.calls.map((c) => c[0])).toEqual([
        "earlier",
        "exact",
        "later",
      ]);
    },
  );

  it("the pad is three coloured buttons; disabled answers nothing", () => {
    const onAnswer = vi.fn();
    render(<AnswerPad variant="pad" onAnswer={onAnswer} disabled />);
    const later = screen.getByRole("button", { name: /Later/ });
    expect(later).toHaveClass("padbtn", "later");
    fireEvent.click(later);
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("shows key hints unless told not to", () => {
    const { rerender, container } = render(<AnswerPad onAnswer={() => {}} />);
    expect(container.querySelectorAll(".kbd")).toHaveLength(3);
    rerender(<AnswerPad onAnswer={() => {}} keys={false} />);
    expect(container.querySelectorAll(".kbd")).toHaveLength(0);
  });
});

describe("controls", () => {
  it("Button variants", () => {
    const onClick = vi.fn();
    render(
      <>
        <Button icon="play" onClick={onClick}>
          Start
        </Button>
        <Button variant="ghost" small>
          Done
        </Button>
      </>,
    );
    const start = screen.getByRole("button", { name: "Start" });
    expect(start).toHaveClass("btn", "primary");
    expect(start).toHaveAttribute("type", "button");
    expect(start.querySelector("svg")).not.toBeNull();
    fireEvent.click(start);
    expect(onClick).toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Done" })).toHaveClass(
      "ghost",
      "small",
    );
  });

  it("IconButton, Card, Pill", () => {
    render(
      <>
        <IconButton icon="x" label="Close" />
        <Card data-testid="card" className="extra" />
        <Pill>Room K7Q4</Pill>
      </>,
    );
    expect(screen.getByRole("button", { name: "Close" })).toHaveClass(
      "icon-btn",
    );
    expect(screen.getByTestId("card")).toHaveClass("card", "extra");
    expect(screen.getByText("Room K7Q4")).toHaveClass("pill");
  });

  it("Chip toggles and shows a count", () => {
    const onClick = vi.fn();
    render(
      <>
        <Chip on count={308} onClick={onClick}>
          Essentials
        </Chip>
        <Chip>Rock</Chip>
      </>,
    );
    const on = screen.getByRole("button", { name: /Essentials/ });
    expect(on).toHaveClass("chip", "on");
    expect(on).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("308").tagName).toBe("SMALL");
    expect(screen.getByRole("button", { name: "Rock" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    fireEvent.click(on);
    expect(onClick).toHaveBeenCalled();
  });

  it("Segmented marks the value and reports a change", () => {
    const onChange = vi.fn();
    render(
      <Segmented
        label="Difficulty"
        value="medium"
        onChange={onChange}
        options={[
          { value: "easy", label: "Easy" },
          { value: "medium", label: "Medium" },
        ]}
      />,
    );
    expect(screen.getByRole("button", { name: "Medium" })).toHaveClass("on");
    fireEvent.click(screen.getByRole("button", { name: "Easy" }));
    expect(onChange).toHaveBeenCalledWith("easy");
  });
});

describe("Avatar", () => {
  it("is the static SVG, named", () => {
    render(<Avatar avatar="mic" size={52} />);
    const img = screen.getByRole("img", { name: "Mic Drop" });
    expect(img).toHaveAttribute("src", "/avatars/mic.svg");
    expect(img).toHaveAttribute("width", "52");
  });

  it("falls back to the first avatar for an unknown key; rings the selected one", () => {
    const { container } = render(<Avatar avatar="kazoo" selected />);
    expect(screen.getByRole("img")).toHaveAttribute(
      "src",
      "/avatars/vinyl.svg",
    );
    expect(container.querySelector(".avatar-pick")).toHaveStyle({
      boxShadow: "0 0 0 3px var(--ink),0 0 0 6px #e9b44c",
    });
  });

  it("AvatarPicker offers all 12 and reports the pick", () => {
    const onChange = vi.fn();
    render(<AvatarPicker value="disco" onChange={onChange} />);
    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(12);
    expect(screen.getByRole("radio", { name: "Disco Dot" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    fireEvent.click(screen.getByRole("radio", { name: "Jukey" }));
    expect(onChange).toHaveBeenCalledWith("jukebox");
  });
});

describe("AwardBadge", () => {
  it("draws each tier with its own gradient", () => {
    const { container } = render(
      <>
        {AWARD_TIERS.map((t) => (
          <AwardBadge key={t} tier={t}>
            {t}
          </AwardBadge>
        ))}
      </>,
    );
    expect(AWARD_TIERS).toEqual([
      "bronze",
      "silver",
      "gold",
      "platinum",
      "diamond",
    ]);
    const ids = [...container.querySelectorAll("linearGradient")].map(
      (g) => g.id,
    );
    expect(new Set(ids).size).toBe(5);
    expect(screen.getByText("gold")).toHaveAttribute("data-tier", "gold");
  });
});

describe("Vinyl, Waveform, ClipTime", () => {
  it("spins on demand", () => {
    const { rerender } = render(<Vinyl />);
    expect(screen.getByTestId("vinyl")).not.toHaveClass("spin");
    rerender(<Vinyl spinning label="1" />);
    expect(screen.getByTestId("vinyl")).toHaveClass("spin");
    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("lights the bars up to the progress and dims with the level", () => {
    const { container } = render(<Waveform progress={0.25} level={0} />);
    expect(container.querySelectorAll("i")).toHaveLength(40);
    expect(container.querySelectorAll("i.on")).toHaveLength(10);
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "25",
    );
    expect(screen.getByRole("progressbar")).toHaveStyle({ opacity: "0.25" });
  });

  it("formats clip time and shows the level meter", () => {
    expect(clock(8.7)).toBe("0:08");
    expect(clock(90)).toBe("1:30");
    expect(clock(Infinity)).toBe("–:––");
    expect(clock(NaN)).toBe("–:––");
    expect(clock(-2)).toBe("0:00");
    const { container, rerender } = render(
      <ClipTime pos={8} total={20} level={0.5} note="live" />,
    );
    expect(screen.getByText("0:08")).toBeInTheDocument();
    expect(screen.getByText("0:20")).toBeInTheDocument();
    expect(container.querySelector(".vb i")).toHaveStyle({ width: "50%" });
    rerender(<ClipTime pos={8} total={20} />);
    expect(container.querySelector(".vb")).toBeNull();
    rerender(<ClipTime pos={0} total={Infinity} level={NaN} />);
    expect(screen.getByText("–:––")).toBeInTheDocument();
  });

  it("shows no progress for an unknown length", () => {
    const { container } = render(<Waveform progress={0 / 0} level={NaN} />);
    expect(container.querySelectorAll("i.on")).toHaveLength(0);
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "0",
    );
  });
});

describe("Cover", () => {
  it("shows the image, and the placeholder when it fails", () => {
    render(
      <Cover
        title="Hey"
        artist="ABBA"
        album="Arrival"
        src="/c.jpg"
        size={100}
      />,
    );
    const img = screen.getByRole("img", { name: "Arrival by ABBA" });
    expect(img.tagName).toBe("IMG");
    fireEvent.error(img);
    const ph = screen.getByRole("img", { name: "Arrival by ABBA" });
    expect(ph.tagName).toBe("DIV");
    expect(ph).toHaveAttribute("data-placeholder");
    expect(ph).toHaveStyle({ width: "100px" });
  });

  it("draws the record sleeve without a cover, remaster suffix dropped", () => {
    render(
      <Cover
        title="Bohemian Rhapsody"
        artist="Queen"
        album="A Night at the Opera (2011 Remaster)"
        mini
      />,
    );
    const ph = screen.getByRole("img", {
      name: "A Night at the Opera by Queen",
    });
    expect(ph).toHaveClass("cover", "mini");
    expect(ph.style.background).toContain("linear-gradient");
    expect(screen.getByText("A Night at the Opera")).toHaveClass("cb");
  });

  it("SongHead lines up cover, title, artist and meta", () => {
    render(
      <SongHead
        cover={<i>c</i>}
        title="Hey Ya!"
        artist="OutKast"
        meta="Hip-Hop"
        center
      />,
    );
    expect(screen.getByText("Hey Ya!")).toHaveClass("song-t");
    expect(screen.getByText("OutKast")).toHaveClass("song-a");
    expect(screen.getByText("Hip-Hop")).toBeInTheDocument();
  });
});

const PLAYERS: Player[] = [
  { name: "Jan", avatar: "vinyl", score: 6 },
  { name: "Mia", avatar: "mic", score: 8, out: true },
  { name: "Luca", avatar: "guitar", score: 5 },
  { name: "Sarah", avatar: "disco", score: 8 },
  { name: "Tom", avatar: "drum", score: 3 },
];

describe("players", () => {
  it("PlayerStrip greys out and crosses the locked-out", () => {
    const { container } = render(<PlayerStrip players={PLAYERS} />);
    const out = container.querySelectorAll("[data-out]");
    expect(out).toHaveLength(1);
    expect(out[0]).toHaveClass("out");
    expect(out[0]).toHaveTextContent("Mia");
    expect(screen.getByLabelText("locked out")).toHaveTextContent("✕");
    expect(screen.getByText("3 pt", { selector: ".s" })).toBeInTheDocument();
  });

  it("tells apart two players of the same name by id", () => {
    const twins: Player[] = [
      { id: "p1", name: "Mia", avatar: "mic", score: 2 },
      { id: "p2", name: "Mia", avatar: "note", score: 5 },
    ];
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    const { container } = render(
      <>
        <PlayerStrip players={twins} />
        <Standings players={twins} />
      </>,
    );
    expect(container.querySelectorAll(".strip .p")).toHaveLength(2);
    expect(container.querySelectorAll(".set-row")).toHaveLength(2);
    expect(err).not.toHaveBeenCalled();
    err.mockRestore();
  });

  it("PlayerStrip can leave the scores out", () => {
    render(<PlayerStrip players={PLAYERS} showScore={false} />);
    expect(screen.queryByText("6 pt")).toBeNull();
  });

  it("Standings sorts by score, the winner first among equals", () => {
    const { container } = render(
      <Standings players={PLAYERS} winner="Sarah" />,
    );
    const names = [...container.querySelectorAll(".set-row")].map(
      (r) => r.querySelectorAll("b")[1].textContent,
    );
    expect(names).toEqual(["Sarah", "Mia", "Jan", "Luca"]);
    expect(screen.getByText("▲")).toBeInTheDocument();
  });

  it("Podium stands 2 · 1 · 3 and lists the rest", () => {
    const { container } = render(<Podium players={PLAYERS} />);
    const places = [...container.querySelectorAll(".place")].map((p) => [
      p.getAttribute("data-place"),
      p.querySelector(".who")!.textContent,
    ]);
    expect(places).toEqual([
      ["2", "Sarah · 8"],
      ["1", "Mia · 8"],
      ["3", "Jan · 6"],
    ]);
    expect(screen.getByText("Luca 5 · Tom 3")).toBeInTheDocument();
  });

  it("Podium copes with fewer than three players", () => {
    const { container } = render(<Podium players={PLAYERS.slice(0, 2)} />);
    expect(container.querySelectorAll(".place")).toHaveLength(2);
    expect(container.querySelector(".podium-rest")).toBeNull();
  });

  it("Confetti", () => {
    const { container } = render(<Confetti />);
    expect(container.querySelectorAll(".confetti i")).toHaveLength(36);
  });
});

describe("AppShell", () => {
  it("shows the tabs and reports a tab change", () => {
    const onTab = vi.fn();
    render(
      <AppShell tab="party" onTab={onTab} footer={<button>Go</button>}>
        <p>content</p>
      </AppShell>,
    );
    expect(screen.getByText("content").parentElement).toHaveClass("scroll");
    expect(
      screen.getByRole("button", { name: "Go" }).parentElement,
    ).toHaveClass("sticky-cta");
    const tabs = screen.getAllByRole("button").filter((b) => b.closest("nav"));
    expect(tabs.map((b) => b.textContent)).toEqual([
      "Home",
      "Party",
      "Train",
      "Settings",
    ]);
    expect(screen.getByRole("button", { name: "Party" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    fireEvent.click(screen.getByRole("button", { name: "Train" }));
    expect(onTab).toHaveBeenCalledWith("train");
  });

  it("has no tabs on screens without a tab", () => {
    render(
      <AppShell scrollClassName="playscr">
        <p>play</p>
      </AppShell>,
    );
    expect(screen.queryByRole("navigation")).toBeNull();
    expect(screen.getByText("play").parentElement).toHaveClass("playscr");
  });

  it("stageZoom scales the 1180×820 box to a landscape screen", () => {
    expect(stageZoom(1180, 820)).toBe(1);
    expect(stageZoom(1920, 1080)).toBeCloseTo(1080 / 820);
    expect(stageZoom(1440, 1200)).toBeCloseTo(1440 / 1180);
    expect(stageZoom(820, 1180)).toBeNull();
  });

  it("StageScale follows the window size", () => {
    const set = (w: number, h: number) => {
      Object.defineProperty(window, "innerWidth", {
        value: w,
        configurable: true,
      });
      Object.defineProperty(window, "innerHeight", {
        value: h,
        configurable: true,
      });
    };
    set(2360, 1640);
    const { container } = render(
      <StageScale>
        <p>stage</p>
      </StageScale>,
    );
    const box = container.querySelector<HTMLElement>(".stage-box")!;
    expect(box.style.zoom).toBe("2");
    set(820, 1180);
    act(() => {
      window.dispatchEvent(new Event("resize"));
    });
    expect(box).toHaveClass("portrait");
    expect(box.style.zoom).toBe("");
    set(1024, 768);
  });
});
