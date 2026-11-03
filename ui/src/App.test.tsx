import { fireEvent, screen, waitFor } from "@testing-library/react";
import { Router } from "wouter";
import { memoryLocation } from "wouter/memory-location";
import { render } from "@testing-library/react";
import App from "./App";
import { audio } from "./test/audio";
import { FakeWorld } from "./test/fakeApi";
import { renderApp } from "./test/render";

describe("App", () => {
  it("shows home for a paired device: game night, today's round, progress, decades, decks", async () => {
    const r = renderApp("/", new FakeWorld().device("p-jan"));
    expect(await screen.findByText("Gold record")).toBeInTheDocument();
    expect(screen.getByText("Play with friends")).toBeInTheDocument();
    expect(screen.getByText("Today's round")).toBeInTheDocument();
    expect(
      await screen.findByText("Card set · 1908–2021 · 308 cards"),
    ).toBeInTheDocument();
    expect(screen.getByText("1,380 / 1,500 XP")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Train the 1970s" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Mega Mix/ }));
    expect(r.path()).toBe("/train");
    expect(JSON.parse(localStorage.getItem("reworm.train")!).deck.sets).toEqual(
      ["megamix"],
    );
  });

  it("takes the tabs and the home buttons to their routes", async () => {
    const r = renderApp("/", new FakeWorld().device("p-jan"));
    const go = (name: string | RegExp, path: string) => {
      fireEvent.click(screen.getAllByRole("button", { name })[0]);
      expect(r.path()).toBe(path);
    };
    await screen.findByText("Gold record");
    go("Party", "/party");
    go(/^Host a game/, "/host");
    await screen.findByLabelText("Room code");
    go("Back", "/party");
    go(/^Join a game/, "/join");
    go("Back", "/party");
    go("Settings", "/settings");
    go("Train", "/train");
    go("Back", "/");
    go("Profile & devices", "/me");
    go("Back", "/");
    go("Change deck", "/train");
    go("Back", "/");
    go("Join", "/join");
  });

  it("shows home on an unpaired device with Join and the way to pair", async () => {
    renderApp("/", new FakeWorld().device());
    expect(
      await screen.findByText("This device isn't paired"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Join" })).toBeInTheDocument();
    expect(screen.queryByText("Today's round")).toBeNull();
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
    fireEvent.click(sw);
    expect(sw).toHaveAttribute("aria-checked", "false");
    expect(localStorage.getItem("reworm.prefetch")).toBe("off");
    await waitFor(() => expect(audio.audios.length).toBe(before + 1));
    fireEvent.click(sw);
    expect(localStorage.getItem("reworm.prefetch")).toBe("on");
    fireEvent.click(screen.getByRole("button", { name: /Profile, avatar/ }));
    expect(r.path()).toBe("/me");
  });
});
