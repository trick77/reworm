import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { FakeWorld } from "../test/fakeApi";
import { renderApp } from "../test/render";

describe("pairing", () => {
  it("claims a device code: the new device logs in as the code's profile", async () => {
    const world = new FakeWorld();
    const mine = world.device("p-mia");
    const { code } = await mine.pair("device");
    const fresh = world.device();
    const r = renderApp(`/pair/${code.toLowerCase()}`, fresh);
    fireEvent.click(screen.getByRole("button", { name: "Pair this device" }));
    await waitFor(() => expect(r.path()).toBe("/"));
    expect((await fresh.me()).profile.name).toBe("Mia");
    expect(
      await screen.findByRole("button", { name: "Profile & devices" }),
    ).toBeInTheDocument();
  });

  it("claims an invite: asks for a name and an avatar, creates the profile", async () => {
    const world = new FakeWorld();
    const { code } = await world.device("p-jan").pair("invite");
    const fresh = world.device();
    const r = renderApp("/pair", fresh);
    fireEvent.change(screen.getByLabelText("Pairing code"), {
      target: { value: code },
    });
    fireEvent.keyDown(screen.getByLabelText("Pairing code"), { key: "Enter" });
    expect(await screen.findByText("You're invited!")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create my profile" }),
    ).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "Lea" },
    });
    fireEvent.click(screen.getByRole("radio", { name: "Quaver" }));
    fireEvent.click(screen.getByRole("button", { name: "Create my profile" }));
    await waitFor(() => expect(r.path()).toBe("/"));
    expect((await fresh.me()).profile).toMatchObject({
      name: "Lea",
      avatar: "note",
    });
  });

  it("shows a full address in the QR when the server hands a path", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    api.pair = () =>
      Promise.resolve({
        code: "H4K9QZ",
        url: "/pair/H4K9QZ",
        expiresAt: Date.now() + 60_000,
      });
    renderApp("/me", api);
    fireEvent.click(
      await screen.findByRole("button", { name: "Add a device" }),
    );
    const panel = await screen.findByRole("dialog", { name: "Add a device" });
    expect(panel.querySelector("[data-qr]")).toHaveAttribute(
      "data-qr",
      `${location.origin}/pair/H4K9QZ`,
    );
    expect(
      within(panel).getByText(/valid 1:00|valid 0:59/),
    ).toBeInTheDocument();
  });

  it("reports an unknown code", async () => {
    const world = new FakeWorld();
    renderApp("/pair/AAAAAA", world.device());
    fireEvent.click(screen.getByRole("button", { name: "Pair this device" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "unknown or expired",
    );
  });
});

describe("setup", () => {
  it("creates the first profile from the setup link, once", async () => {
    const world = new FakeWorld({ seed: false });
    const first = world.device();
    const r = renderApp("/setup?token=fake-setup", first);
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "Jan" },
    });
    fireEvent.click(screen.getByRole("radio", { name: /Disco Dot/ }));
    fireEvent.click(screen.getByRole("button", { name: "Create my profile" }));
    await waitFor(() => expect(r.path()).toBe("/"));
    expect((await first.me()).profile).toMatchObject({
      name: "Jan",
      avatar: "disco",
      admin: true,
    });
    r.unmount();

    renderApp("/setup?token=fake-setup", world.device());
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "Eve" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create my profile" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("already used");
  });

  it("says when the link has no token", () => {
    renderApp("/setup", new FakeWorld().device());
    expect(screen.getByRole("alert")).toHaveTextContent("no token");
  });
});

describe("profiles and devices", () => {
  it("switches the profile on a shared device", async () => {
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const r = renderApp("/who", api);
    fireEvent.click(await screen.findByRole("button", { name: "Sarah" }));
    await waitFor(() => expect(r.path()).toBe("/"));
    expect((await api.me()).profile.name).toBe("Sarah");
  });

  it("Add someone opens the profile page with an invite code", async () => {
    const world = new FakeWorld();
    const r = renderApp("/who", world.device("p-jan"));
    fireEvent.click(await screen.findByRole("button", { name: "Add someone" }));
    expect(r.path()).toBe("/me?invite=1");
    expect(
      await screen.findByRole("dialog", { name: "Invite someone" }),
    ).toBeInTheDocument();
  });

  it("changes the avatar, lists and removes devices, makes a pairing code", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const world = new FakeWorld();
    const api = world.device("p-jan");
    renderApp("/me", api);
    expect(await screen.findByText("Profile & devices")).toBeInTheDocument();
    expect(await screen.findByText("this one")).toBeInTheDocument();
    expect(screen.getByText("2 h ago")).toBeInTheDocument();
    expect(screen.getByText("3 days ago")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: /Woofer/ }));
    await waitFor(() =>
      expect(world.profiles.get("p-jan")!.avatar).toBe("speaker"),
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Remove MacBook · Safari" }),
    );
    await waitFor(() =>
      expect(screen.queryByText("MacBook · Safari")).toBeNull(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Add a device" }));
    const panel = await screen.findByRole("dialog", { name: "Add a device" });
    const code = [...world.pairs.keys()][0];
    expect(within(panel).getByText(code)).toBeInTheDocument();
    expect(
      within(panel).getByText("valid 5:00 · works once"),
    ).toBeInTheDocument();
    expect(panel.querySelector("[data-qr]")).toHaveAttribute(
      "data-qr",
      `${location.origin}/pair/${code}`,
    );
    await act(() => vi.advanceTimersByTimeAsync(5 * 60_000 + 1000));
    expect(within(panel).getByText(/expired/)).toBeInTheDocument();
    fireEvent.click(within(panel).getByRole("button", { name: "Done" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Switch profile on this device" }),
    );
    expect(await screen.findByText("Who's playing?")).toBeInTheDocument();
    vi.useRealTimers();
  });

  it("an unpaired device sees how to pair on the profile page and Who's playing", async () => {
    const world = new FakeWorld();
    const r = renderApp("/me", world.device());
    expect(
      await screen.findByText("This device isn't paired"),
    ).toBeInTheDocument();
    r.unmount();
    const w = renderApp("/who", world.device());
    fireEvent.click(
      await screen.findByRole("button", { name: "Pair this device" }),
    );
    expect(w.path()).toBe("/pair");
  });
});
