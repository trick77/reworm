import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { ApiError } from "../../api/client";
import { audio } from "../../test/audio";
import { FAKE_SONGS, FakeWorld } from "../../test/fakeApi";
import { renderApp } from "../../test/render";
import { FLY } from "../../components/YearCard";
import { YEAR_MAX, YEAR_MIN } from "../../lib/prefs";

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
    // the cover is the link the server sent (it names the cover), never one the UI builds from the id
    const covers = [...document.querySelectorAll("img")].map((i) => i.src);
    expect(first.song.art && covers.includes(first.song.art)).toBe(true);
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

  it("shows progress, streak and decades on top of the Train tab; a decade, the weakest and all are one-offs", async () => {
    const world = new FakeWorld();
    const start = async (name: string, from: number, to: number) => {
      const r = renderApp("/train", world.device("p-jan"));
      expect(await screen.findByText("Your progress")).toBeInTheDocument();
      expect(screen.getByText("Gold record")).toBeInTheDocument();
      expect(screen.getByText("1,380 / 1,500 XP")).toBeInTheDocument();
      expect(screen.getByText("12")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name }));
      await waitFor(() => expect(r.path()).toMatch(/^\/train\/t-/));
      const ses = [...world.sessions.values()].at(-1)!;
      expect(ses.settings.deck).toMatchObject({ from, to });
      r.unmount();
    };
    await start("Train the 1970s", 1970, 1979);
    await start("Train weakest →", 1970, 1979);
    await start("Train all decades", YEAR_MIN, YEAR_MAX);
    // a decade is a one-off: no training defaults saved
    expect(localStorage.getItem("reworm.train")).toBeNull();
  });

  it("starts a decade with the settings in the form, not the saved ones", async () => {
    const world = new FakeWorld();
    const r = renderApp("/train", world.device("p-jan"));
    expect(await screen.findByText("Your progress")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Genres" }));
    fireEvent.click(screen.getByRole("button", { name: /^Rock/ }));
    fireEvent.click(screen.getByRole("button", { name: /Pin the year/ }));
    fireEvent.click(screen.getByRole("button", { name: "Hard" }));
    fireEvent.click(screen.getByRole("button", { name: "40 songs" }));
    fireEvent.click(screen.getByRole("button", { name: "Train the 1970s" }));
    await waitFor(() => expect(r.path()).toMatch(/^\/train\/t-/));
    const ses = [...world.sessions.values()].at(-1)!;
    expect(ses.settings).toMatchObject({
      mode: "pin",
      difficulty: "hard",
      length: 40,
      deck: { genres: ["rockg"], from: 1970, to: 1979 },
    });
    expect(localStorage.getItem("reworm.train")).toBeNull();
  });

  it("shows the Train tab only once progress is in, so nothing pushes the form down", async () => {
    const api = new FakeWorld().device("p-jan");
    const real = api.trainStats;
    let answer = () => {};
    api.trainStats = () =>
      new Promise((resolve) => {
        answer = () => void real().then(resolve);
      });
    renderApp("/train", api);
    await waitFor(() => expect(api.calls).toContain("decks"));
    await act(() => new Promise((r) => setTimeout(r, 20)));
    expect(screen.queryByRole("button", { name: /Mega Mix/ })).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
    answer();
    expect(await screen.findByText("Your progress")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Mega Mix/ }),
    ).toBeInTheDocument();
  });

  it("starts one session at a time: tiles and Start wait while one starts", async () => {
    const api = new FakeWorld().device("p-jan");
    const real = api.startTraining;
    let starts = 0;
    let fail = () => {};
    api.startTraining = (s) => {
      starts++;
      if (starts > 1) return real(s);
      return new Promise((_, reject) => {
        fail = () => reject(new ApiError(503, "down", "start down"));
      });
    };
    const r = renderApp("/train", api);
    expect(await screen.findByText("Your progress")).toBeInTheDocument();
    const buttons = [
      "Train the 1970s",
      "Train all decades",
      "Train weakest →",
      "Start · 10 songs",
    ];
    fireEvent.click(screen.getByRole("button", { name: "Train the 1970s" }));
    for (const name of buttons)
      expect(screen.getByRole("button", { name })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Start · 10 songs" }));
    fireEvent.click(screen.getByRole("button", { name: "Train all decades" }));
    expect(starts).toBe(1);
    // a failed start says so once and frees them all again
    await act(async () => fail());
    expect(await screen.findByRole("alert")).toHaveTextContent("start down");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    for (const name of buttons)
      expect(screen.getByRole("button", { name })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Start · 10 songs" }));
    await waitFor(() => expect(r.path()).toMatch(/^\/train\/t-/));
  });

  it("shows a stats failure on the Train tab with a retry, the setup still usable", async () => {
    const api = new FakeWorld().device("p-jan");
    const real = api.trainStats;
    let down = true;
    api.trainStats = () =>
      down ? Promise.reject(new ApiError(503, "down", "stats down")) : real();
    renderApp("/train", api);
    expect(await screen.findByText("stats down")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "20 songs" }));
    down = false;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Your progress")).toBeInTheDocument();
    // the form kept its edits through the retry
    expect(
      screen.getByRole("button", { name: "Start · 20 songs" }),
    ).toBeInTheDocument();
  });

  it("starts today's round from home, and takes a code on a device without a profile", async () => {
    const world = new FakeWorld();
    const r = renderApp("/", world.device("p-jan"));
    fireEvent.click(
      await screen.findByRole("button", { name: "Start training" }),
    );
    await waitFor(() => expect(r.path()).toMatch(/^\/train\/t-/));
    const ses = [...world.sessions.values()].at(-1)!;
    r.unmount();
    const r3 = renderApp("/train", world.device());
    expect(
      await screen.findByText("Training needs a profile"),
    ).toBeInTheDocument();
    const { code } = await world.device("p-jan").pair();
    fireEvent.click(screen.getByRole("button", { name: /Use its code/ }));
    fireEvent.change(screen.getByLabelText("Your code"), {
      target: { value: code },
    });
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    expect(
      await screen.findByRole("button", { name: /Mega Mix/ }),
    ).toBeInTheDocument();
    r3.unmount();

    // a session opened on a device without a profile: the code, then the session itself
    const phone = world.device();
    renderApp(`/train/${ses.id}`, phone);
    expect(
      await screen.findByText("Training needs a profile"),
    ).toBeInTheDocument();
    const again = await world.device("p-jan").pair();
    fireEvent.click(screen.getByRole("button", { name: /Use its code/ }));
    fireEvent.change(screen.getByLabelText("Your code"), {
      target: { value: again.code },
    });
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    expect(
      await screen.findByRole("button", { name: "End session" }),
    ).toBeInTheDocument();
  });
});
