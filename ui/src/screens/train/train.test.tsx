import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { ApiError } from "../../api/client";
import { audio } from "../../test/audio";
import { FAKE_SONGS, FakeWorld } from "../../test/fakeApi";
import { renderApp } from "../../test/render";
import { FLY } from "../../components/YearCard";

const fly = () => act(() => new Promise((r) => setTimeout(r, FLY + 20)));

describe("training", () => {
  it("sets up a session, answers, reveals, goes on to the next card and the summary", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const r = renderApp("/train", api);
    expect(await screen.findByText("New session")).toBeInTheDocument();
    // pick decks, years, mode details, then start
    fireEvent.click(await screen.findByRole("button", { name: /Mega Mix/ }));
    fireEvent.click(screen.getByRole("button", { name: "Categories" }));
    fireEvent.click(screen.getByRole("button", { name: /Smash Hits/ }));
    fireEvent.click(screen.getByRole("button", { name: "Genres" }));
    fireEvent.click(screen.getByRole("button", { name: /^Rock/ }));
    fireEvent.click(screen.getByRole("button", { name: "60s–80s" }));
    expect(screen.getByText("1960 – 1989")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("From year"), {
      target: { value: "1970" },
    });
    fireEvent.change(screen.getByLabelText("To year"), {
      target: { value: "1995" },
    });
    expect(screen.getByText("1970 – 1995")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Hard" }));
    expect(
      screen.getByText("Lots of ±1–3. Game-night pressure."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Random spot" }));
    fireEvent.click(screen.getByRole("button", { name: "30 s" }));
    fireEvent.click(screen.getByRole("button", { name: "20 songs" }));
    fireEvent.click(screen.getByRole("button", { name: "Start · 20 songs" }));
    await waitFor(() => expect(r.path()).toMatch(/^\/train\/t-/));
    const saved = JSON.parse(localStorage.getItem("reworm.train")!);
    expect(saved).toMatchObject({
      mode: "years",
      difficulty: "hard",
      clip: { length: 30, start: "random" },
      length: 20,
      deck: {
        sets: ["essentials", "megamix"],
        categories: ["smash"],
        genres: ["rockg"],
        from: 1970,
        to: 1995,
      },
    });

    // play: the clip starts, the year card answers
    const ses = [...world.sessions.values()][0];
    const first = ses.queue[0];
    expect(
      await screen.findByText(String(first.card.proposedYear)),
    ).toBeInTheDocument();
    expect(
      audio.audios.some((a) => a.src === first.song.preview && !a.paused),
    ).toBe(true);
    const y = first.song.year;
    const shown = first.card.proposedYear!;
    const right = y === shown ? " " : y < shown ? "ArrowLeft" : "ArrowRight";
    fireEvent.keyDown(window, { key: right });
    await fly();
    expect(await screen.findByText(first.song.title)).toBeInTheDocument();
    expect(screen.getByText(/XP/)).toBeInTheDocument();
    expect(screen.getByTestId("ruler")).toBeInTheDocument();
    expect(screen.getByText(`First released ${y}`)).toBeInTheDocument();

    // next card by Enter, then a wrong answer by the button: miss, comes back later
    fireEvent.keyDown(window, { key: "Enter" });
    const second = ses.queue[1];
    expect(
      await screen.findByText(String(second.card.proposedYear)),
    ).toBeInTheDocument();
    const y2 = second.song.year;
    const s2 = second.card.proposedYear!;
    fireEvent.click(
      screen.getByRole("button", { name: y2 < s2 ? /Later/ : /Earlier/ }),
    );
    await fly();
    expect(await screen.findByText("MISS")).toBeInTheDocument();
    expect(
      screen.getByText("Back in a few cards with a different year."),
    ).toBeInTheDocument();

    // end the session early: the summary
    fireEvent.click(screen.getByRole("button", { name: /Next song/ }));
    fireEvent.click(await screen.findByRole("button", { name: "End session" }));
    expect(await screen.findByText("Session complete")).toBeInTheDocument();
    expect(await screen.findByText("1 of 2 right")).toBeInTheDocument();
    expect(screen.getByText("To learn (1)")).toBeInTheDocument();
    expect(screen.getByText(second.song.title)).toBeInTheDocument();
    expect(await screen.findByText(/XP to platinum/)).toBeInTheDocument();
    // drill the misses: exactly those songs, the saved defaults stay
    const defaults = localStorage.getItem("reworm.train");
    fireEvent.click(screen.getByRole("button", { name: "Drill misses (1)" }));
    await waitFor(() => expect(world.sessions.size).toBe(2));
    const drill = [...world.sessions.values()][1];
    expect(drill.settings.songIds).toEqual([second.song.id]);
    expect(drill.queue.map((q) => q.song.id)).toEqual([second.song.id]);
    expect(localStorage.getItem("reworm.train")).toBe(defaults);
  });

  it("pins the year, resumes a session on reload, and runs to its end", async () => {
    const world = new FakeWorld();
    world.songs = FAKE_SONGS.slice(0, 2);
    const api = world.device("p-jan");
    const { id } = await api.startTraining({
      deck: { sets: [], categories: [], genres: [], from: 1900, to: 2025 },
      mode: "pin",
      difficulty: "medium",
      clip: { length: 20, start: "chorus" },
      length: 2,
    });
    renderApp(`/train/${id}`, api);
    const dial = await screen.findByLabelText("Your year");
    fireEvent.change(dial, { target: { value: String(world.songs[0].year) } });
    fireEvent.click(screen.getByRole("button", { name: /Lock it in/ }));
    expect(await screen.findByText("Exact year!")).toBeInTheDocument();
    expect(screen.getByText(/BULLS/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Next song/ }));
    const dial2 = await screen.findByLabelText("Your year");
    fireEvent.change(dial2, {
      target: { value: String(world.songs[1].year + 5) },
    });
    fireEvent.keyDown(window, { key: "Enter" });
    expect(await screen.findByText("Off by 5 years")).toBeInTheDocument();
    // the miss comes back as a retry card: then the session ends
    fireEvent.click(screen.getByRole("button", { name: /Next song/ }));
    const dial3 = await screen.findByLabelText("Your year");
    fireEvent.change(dial3, {
      target: { value: String(world.songs[1].year + 1) },
    });
    fireEvent.click(screen.getByRole("button", { name: /Lock it in/ }));
    expect(await screen.findByText("Within a year")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /See results/ }));
    expect(await screen.findByText("2 of 3 right")).toBeInTheDocument();
    expect(
      screen.getByText(`you: ${world.songs[1].year + 5}`),
    ).toBeInTheDocument();
  });

  it("offers another round after a session without misses", async () => {
    const world = new FakeWorld();
    world.songs = FAKE_SONGS.slice(0, 1);
    const api = world.device("p-jan");
    const settings = {
      deck: { sets: [], categories: [], genres: [], from: 1900, to: 2100 },
      mode: "pin" as const,
      difficulty: "medium" as const,
      clip: { length: 20, start: "chorus" as const },
      length: 1,
    };
    localStorage.setItem("reworm.train", JSON.stringify(settings));
    const { id } = await api.startTraining(settings);
    renderApp(`/train/${id}`, api);
    fireEvent.change(await screen.findByLabelText("Your year"), {
      target: { value: String(world.songs[0].year) },
    });
    fireEvent.click(screen.getByRole("button", { name: /Lock it in/ }));
    fireEvent.click(await screen.findByRole("button", { name: /See results/ }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Another round" }),
    );
    await waitFor(() => expect(world.sessions.size).toBe(2));
  });

  it("drills each missed song once, however often it was missed", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const { id } = await api.startTraining({
      deck: { sets: [], categories: [], genres: [], from: 1900, to: 2100 },
      mode: "pin",
      difficulty: "medium",
      clip: { length: 20, start: "chorus" },
      length: 2,
    });
    const real = api.trainSummary;
    api.trainSummary = async (sid) => {
      const s = await real(sid);
      const song = FAKE_SONGS[0];
      const miss = { song: { ...song, coverUrl: null }, proposedYear: null };
      return {
        ...s,
        misses: [
          { ...miss, choice: 1900 },
          { ...miss, choice: 1901 },
          {
            song: { ...FAKE_SONGS[1], coverUrl: null },
            proposedYear: null,
            choice: 1900,
          },
        ],
      };
    };
    renderApp(`/train/${id}`, api);
    fireEvent.click(await screen.findByRole("button", { name: "End session" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Drill misses (2)" }),
    );
    await waitFor(() => expect(world.sessions.size).toBe(2));
    const drill = [...world.sessions.values()][1];
    expect(drill.settings.songIds).toEqual([
      FAKE_SONGS[0].id,
      FAKE_SONGS[1].id,
    ]);
    expect(drill.settings.length).toBe(2);
  });

  it("offers to try again when the server says the next card failed (nextError), and resumes at the session's card", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const { id } = await api.startTraining({
      deck: { sets: [], categories: [], genres: [], from: 1900, to: 2100 },
      mode: "pin",
      difficulty: "medium",
      clip: { length: 20, start: "chorus" },
      length: 3,
    });
    const real = api.trainAnswer;
    api.trainAnswer = async (sid, b) => ({
      ...(await real(sid, b)),
      next: null,
      nextError: "previews_unavailable",
    });
    const realSession = api.trainSession;
    let down = true;
    api.trainSession = (sid) =>
      down
        ? Promise.reject(new ApiError(503, "previews_unavailable", "no clip"))
        : realSession(sid);
    down = false;
    renderApp(`/train/${id}`, api);
    fireEvent.click(await screen.findByRole("button", { name: /Lock it in/ }));
    expect(
      await screen.findByText(/The next song didn't load/),
    ).toBeInTheDocument();
    down = true;
    fireEvent.click(screen.getByRole("button", { name: /Try again/ }));
    expect(await screen.findByText(/no clip/)).toBeInTheDocument();
    down = false;
    fireEvent.keyDown(window, { key: "Enter" });
    expect(await screen.findByLabelText("Your year")).toBeInTheDocument();
    expect(screen.getByLabelText("Song 2 of 3")).toBeInTheDocument();
  });

  it("plays, pauses and replays the clip by buttons and keys", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const { id } = await api.startTraining({
      deck: { sets: [], categories: [], genres: [], from: 1900, to: 2025 },
      mode: "years",
      difficulty: "easy",
      clip: { length: 20, start: "chorus" },
      length: 3,
    });
    renderApp(`/train/${id}`, api);
    fireEvent.click(await screen.findByRole("button", { name: "Pause" }));
    await act(() => new Promise((r) => setTimeout(r, 400)));
    fireEvent.click(await screen.findByRole("button", { name: "Play" }));
    expect(
      await screen.findByRole("button", { name: "Pause" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Replay" }));
    fireEvent.keyDown(window, { key: "p" });
    await act(() => new Promise((r) => setTimeout(r, 400)));
    fireEvent.keyDown(window, { key: "p" });
    fireEvent.keyDown(window, { key: "r" });
    expect(screen.getByRole("button", { name: "Pause" })).toBeInTheDocument();
  });

  it("starts today's round from home, and a decade, and points an unpaired device to pairing", async () => {
    const world = new FakeWorld();
    const r = renderApp("/", world.device("p-jan"));
    fireEvent.click(await screen.findByRole("button", { name: "Start" }));
    await waitFor(() => expect(r.path()).toMatch(/^\/train\/t-/));
    r.unmount();
    const r2 = renderApp("/", world.device("p-jan"));
    fireEvent.click(
      await screen.findByRole("button", { name: "Train the 1970s" }),
    );
    await waitFor(() => expect(r2.path()).toMatch(/^\/train\/t-/));
    const ses = [...world.sessions.values()].at(-1)!;
    expect(ses.settings.deck).toMatchObject({ from: 1970, to: 1979 });
    // a decade is a one-off: the saved defaults are still today's round
    expect(
      JSON.parse(localStorage.getItem("reworm.train")!).deck.from,
    ).not.toBe(1970);
    r2.unmount();
    renderApp("/train", world.device());
    expect(
      await screen.findByText("This device isn't paired"),
    ).toBeInTheDocument();
  });
});
