import { fireEvent, screen, waitFor } from "@testing-library/react";
import { Router } from "wouter";
import { memoryLocation } from "wouter/memory-location";
import { render } from "@testing-library/react";
import App from "./App";
import { ApiError } from "./api/client";
import { player } from "./audio/player";
import { audio } from "./test/audio";
import { FakeWorld } from "./test/fakeApi";
import { renderApp } from "./test/render";

describe("App", () => {
  it("shows home for a paired device: the game in three steps, then only solo training", async () => {
    renderApp("/", new FakeWorld().device("p-jan"));
    expect(await screen.findByText("Essentials")).toBeInTheDocument();
    expect(screen.getByText("Play with friends")).toBeInTheDocument();
    const steps = screen.getAllByRole("listitem").map((li) => li.textContent);
    expect(steps).toEqual([
      "1One screen plays a song clip and shows a year.",
      "2Everyone taps Earlier, Spot on or Later on their phone.",
      "3Fastest right answer takes the round.",
    ]);
    expect(screen.getByText("Solo training")).toBeInTheDocument();
    // progress, decades, streak and decks live on the Train tab
    for (const gone of ["Your progress", "Decades", "Decks", "Gold record"])
      expect(screen.queryByText(gone)).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Train the 1970s" }),
    ).toBeNull();
  });

  it("has the tabs Home, Train and Settings, and sends the old /party home", async () => {
    const r = renderApp("/party", new FakeWorld().device("p-jan"));
    expect(r.path()).toBe("/");
    expect(await screen.findByText("Solo training")).toBeInTheDocument();
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(
      [...nav.querySelectorAll("button")].map((b) => b.textContent),
    ).toEqual(["Home", "Train", "Settings"]);
  });

  it("takes the tabs and the home buttons to their routes", async () => {
    const r = renderApp("/", new FakeWorld().device("p-jan"));
    const go = (name: string | RegExp, path: string) => {
      fireEvent.click(screen.getAllByRole("button", { name })[0]);
      expect(r.path()).toBe(path);
    };
    await screen.findByText("Solo training");
    go(/^Host a game/, "/host");
    await screen.findByLabelText("Room code");
    go("Back", "/");
    go(/^Join$/, "/join");
    go("Back", "/");
    go("Settings", "/settings");
    go("Train", "/train");
    go("Back", "/");
    go("Profile & devices", "/me");
    go("Back", "/");
    go("Change deck", "/train");
    go("Back", "/");
    go("Join", "/join");
  });

  it("leads with Host on a paired device", async () => {
    renderApp("/", new FakeWorld().device("p-jan"));
    await screen.findByText("Solo training");
    const host = screen.getByRole("button", { name: "Host a game" });
    const join = screen.getByRole("button", { name: "Join" });
    expect(
      host.compareDocumentPosition(join) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Create one" })).toBeNull();
  });

  it("leads with Join on an unpaired device, a new profile an optional link", async () => {
    const r = renderApp("/", new FakeWorld().device());
    await screen.findByRole("button", { name: "Create one" });
    expect(
      screen.getByText(/Hosting and solo training need a profile/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/pair/i)).toBeNull();
    expect(screen.queryByText(/server/i)).toBeNull();
    // the same three steps as on a paired device
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    const join = screen.getByRole("button", { name: "Join a game" });
    const host = screen.getByRole("button", { name: "Host" });
    expect(
      join.compareDocumentPosition(host) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByText("Solo training")).toBeNull();
    expect(screen.queryByText(/invite/i)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Create one" }));
    expect(r.path()).toBe("/me");
    expect(
      await screen.findByRole("button", { name: "Create my profile" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    fireEvent.click(await screen.findByRole("button", { name: "Join a game" }));
    expect(r.path()).toBe("/join");
  });

  it("on a fresh install points to the setup link, no game buttons", async () => {
    const world = new FakeWorld({ seed: false });
    const r = renderApp("/", world.device());
    expect(
      await screen.findByText("Reworm isn't set up yet"),
    ).toBeInTheDocument();
    expect(screen.getByText(/docker logs/)).toBeInTheDocument();
    expect(screen.getByText("Play with friends")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Host/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Join/ })).toBeNull();
    expect(screen.queryByText(/Create one/)).toBeNull();
    expect(screen.queryByText(/server/i)).toBeNull();
    r.unmount();
    // the code screen too: there is nobody to get a code from yet
    renderApp("/pair", world.device());
    expect(
      await screen.findByText("Reworm isn't set up yet"),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Your code")).toBeNull();
  });

  it("on a fresh install looks again when the app comes back to the front", async () => {
    const world = new FakeWorld({ seed: false });
    const pad = world.device();
    const me = () => pad.calls.filter((c) => c === "me").length;
    renderApp("/", pad);
    expect(
      await screen.findByText("Reworm isn't set up yet"),
    ).toBeInTheDocument();
    // still not set up: asked once more, the card stays put meanwhile
    const before = me();
    fireEvent(document, new Event("visibilitychange"));
    expect(screen.getByText("Reworm isn't set up yet")).toBeInTheDocument();
    await waitFor(() => expect(me()).toBe(before + 1));
    expect(screen.getByText("Reworm isn't set up yet")).toBeInTheDocument();
    // set up on another device meanwhile: back on the front, the game is there
    await world
      .device()
      .setup({ token: "fake-setup", name: "Jan", avatar: "vinyl" });
    fireEvent.focus(window);
    expect(
      await screen.findByRole("button", { name: "Join a game" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Reworm isn't set up yet")).toBeNull();
    // with an answer in, coming back asks nothing
    const after = me();
    fireEvent.focus(window);
    fireEvent(document, new Event("visibilitychange"));
    await new Promise((r) => setTimeout(r, 20));
    expect(me()).toBe(after);
  });

  it("on a fresh install checks again on a tap", async () => {
    const world = new FakeWorld({ seed: false });
    renderApp("/pair", world.device());
    expect(
      await screen.findByText("Reworm isn't set up yet"),
    ).toBeInTheDocument();
    await world
      .device()
      .setup({ token: "fake-setup", name: "Jan", avatar: "vinyl" });
    fireEvent.click(screen.getByRole("button", { name: "Check again" }));
    expect(await screen.findByLabelText("Your code")).toBeInTheDocument();
  });

  it("holds the game night buttons back until it knows paired or not", async () => {
    const api = new FakeWorld().device();
    let answer = () => {};
    const real = api.me;
    api.me = () =>
      new Promise<never>((_, reject) => {
        answer = () => void real().catch(reject);
      });
    renderApp("/", api);
    expect(screen.getByText("Play with friends")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Host/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Join/ })).toBeNull();
    answer();
    expect(
      await screen.findByRole("button", { name: "Join a game" }),
    ).toBeInTheDocument();
  });

  it("says when a page doesn't exist", () => {
    const r = renderApp("/nope", new FakeWorld().device());
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Home" }));
    expect(r.path()).toBe("/");
  });

  it("serves the gallery on /gallery in dev only", async () => {
    const world = new FakeWorld();
    const loc = memoryLocation({ path: "/gallery" });
    const { unmount } = render(
      <Router hook={loc.hook}>
        <App api={world.device()} dev />
      </Router>,
    );
    expect(await screen.findByText("component gallery")).toBeInTheDocument();
    unmount();
    renderApp("/gallery", world.device());
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
  });
});

describe("Settings", () => {
  it("switches the audio engine and the prefetch, shows the version and the profile", async () => {
    const r = renderApp("/settings", new FakeWorld().device("p-jan"));
    expect(
      await screen.findByText(
        "Reworm 0.0.0-fake · self-hosted, Apple Music previews.",
      ),
    ).toBeInTheDocument();
    expect(await screen.findByText(/3 devices/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Plain" }));
    expect(screen.getByRole("button", { name: "Plain" })).toHaveClass("on");
    expect(localStorage.getItem("reworm.audioEngine")).toBe("element");
    const sw = screen.getByRole("switch", { name: "Preload the next clip" });
    expect(sw).toHaveAttribute("aria-checked", "true");
    const before = audio.audios.length;
    const p = player();
    fireEvent.click(sw);
    expect(sw).toHaveAttribute("aria-checked", "false");
    expect(localStorage.getItem("reworm.prefetch")).toBe("off");
    // in place: the same player, one fresh element, the AudioContext kept
    expect(player()).toBe(p);
    expect(p.prefetching).toBe(false);
    await waitFor(() => expect(audio.audios.length).toBe(before + 1));
    const ctxs = audio.ctxs.length;
    fireEvent.click(sw);
    expect(localStorage.getItem("reworm.prefetch")).toBe("on");
    expect(p.prefetching).toBe(true);
    expect(audio.audios.length).toBe(before + 3);
    expect(audio.ctxs.length).toBe(ctxs);
    fireEvent.click(screen.getByRole("button", { name: /Profile, avatar/ }));
    expect(r.path()).toBe("/me");
  });
});

describe("a server that fails /api/me (not 401)", () => {
  it("shows the error with a retry instead of a spinner, and recovers", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const real = api.me;
    let down = true;
    api.me = () =>
      down ? Promise.reject(new ApiError(503, "down", "server down")) : real();
    renderApp("/train", api);
    expect(await screen.findByText("server down")).toBeInTheDocument();
    down = false;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("button", { name: /Mega Mix/ }),
    ).toBeInTheDocument();
  });

  it("on home and the profile page too", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    api.me = () => Promise.reject(new ApiError(503, "down", "server down"));
    const r = renderApp("/", api);
    expect(await screen.findByText("server down")).toBeInTheDocument();
    // no profile known: the guest layout, Join first, as on an unpaired phone
    expect(
      screen.getByRole("button", { name: "Join a game" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Host a game" })).toBeNull();
    r.unmount();
    renderApp("/me", api);
    expect(await screen.findByText("server down")).toBeInTheDocument();
  });
});
