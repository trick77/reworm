import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { ApiError } from "../api/client";
import { FakeWorld } from "../test/fakeApi";
import { renderApp } from "../test/render";

describe("pairing", () => {
  it("claims a device code: the new device logs in as the code's profile", async () => {
    const world = new FakeWorld();
    const mine = world.device("p-mia");
    const { code } = await mine.pair();
    const fresh = world.device();
    const r = renderApp(`/pair/${code.toLowerCase()}`, fresh);
    expect(screen.getByText("Enter your code")).toBeInTheDocument();
    expect(screen.queryByText(/invite/i)).toBeNull();
    expect(screen.getByText(/Add a device/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    await waitFor(() => expect(r.path()).toBe("/"));
    expect((await fresh.me()).profile.name).toBe("Mia");
    expect(
      await screen.findByRole("button", { name: "Profile & devices" }),
    ).toBeInTheDocument();
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
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "unknown or expired",
    );
  });
});

describe("setup", () => {
  it("creates the first profile from the setup link, once", async () => {
    const world = new FakeWorld({ seed: false });
    const first = world.device();
    // the link exactly as the server log prints it: /setup/<token>
    const r = renderApp("/setup/fake-setup", first);
    expect(
      screen.getByText(
        "The first profile on this Reworm. You'll be its admin.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/server|pair/i)).toBeNull();
    // form and button a centered block on a wide screen
    expect(r.container.querySelector(".scroll")).toHaveClass("form");
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
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This setup link was already used. Get a code from a device that has a profile.",
    );
  });

  it("says when the link is wrong, or has no token: copy it from the container log", async () => {
    const world = new FakeWorld({ seed: false });
    const r = renderApp("/setup/wrong", world.device());
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "Jan" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create my profile" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This setup link isn't valid. Copy it again from the container log.",
    );
    r.unmount();
    renderApp("/setup", world.device());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "The setup link has no token. Open the link from the container log.",
    );
  });
});

describe("profiles and devices", () => {
  it("changes the avatar, lists and removes devices, makes a pairing code", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const world = new FakeWorld();
    const api = world.device("p-jan");
    const r = renderApp("/me", api);
    expect(await screen.findByText("Profile & devices")).toBeInTheDocument();
    expect(await screen.findByText("this one")).toBeInTheDocument();
    expect(screen.getByText("2 h ago")).toBeInTheDocument();
    expect(screen.getByText("3 days ago")).toBeInTheDocument();
    // profiles are private: no list of the others, no invite
    expect(screen.queryByText("Mia")).toBeNull();
    expect(screen.queryByText(/invite/i)).toBeNull();

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
      screen.getByRole("button", {
        name: "Use another profile on this device",
      }),
    );
    expect(r.path()).toBe("/pair");
    // a centered block on a wide screen
    expect(document.querySelector(".scroll")).toHaveClass("form");
    vi.useRealTimers();
  });

  it("a device without a profile creates one right on the profile page", async () => {
    const world = new FakeWorld();
    const fresh = world.device();
    const r = renderApp("/me", fresh);
    expect(
      await screen.findByText("No profile on this device yet"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/invite|pair/i)).toBeNull();
    const create = screen.getByRole("button", { name: "Create my profile" });
    expect(create).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: " Lea " },
    });
    fireEvent.click(screen.getByRole("radio", { name: "Quaver" }));
    fireEvent.click(create);
    expect(await screen.findByText("this one")).toBeInTheDocument();
    expect(r.path()).toBe("/me");
    expect((await fresh.me()).profile).toMatchObject({
      name: "Lea",
      avatar: "note",
      admin: false,
    });
  });

  it("a device without a profile takes the code of its other device instead", async () => {
    const world = new FakeWorld();
    const r = renderApp("/me", world.device());
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Have a profile on another device? Use its code",
      }),
    );
    expect(
      screen.getByText(/Add a device and enter its code/),
    ).toBeInTheDocument();
    // a code of Mia's other device: the page goes on as Mia's
    const { code } = await world.device("p-mia").pair();
    fireEvent.change(screen.getByLabelText("Your code"), {
      target: { value: code },
    });
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    expect(await screen.findByText("this one")).toBeInTheDocument();
    expect(r.path()).toBe("/me");
  });

  it("a profile's only device asks before it leaves: keep it, or delete it and go on", async () => {
    const world = new FakeWorld();
    const mia = world.device("p-mia");
    const r = renderApp("/me", mia);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Use another profile on this device",
      }),
    );
    expect(r.path()).toBe("/pair");
    expect(
      await screen.findByText("Use another profile instead of Mia"),
    ).toBeInTheDocument();
    // Jan's code: Mia would be stranded
    const { code } = await world.device("p-jan").pair();
    fireEvent.change(screen.getByLabelText("Your code"), {
      target: { value: code },
    });
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This is Mia's only device. Going on deletes Mia's profile and training progress for good.",
    );
    // another code is another profile: the question goes, asked again on the next try
    fireEvent.change(screen.getByLabelText("Your code"), {
      target: { value: "AAAAAA" },
    });
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.change(screen.getByLabelText("Your code"), {
      target: { value: code },
    });
    fireEvent.click(screen.getByRole("button", { name: "Use this code" }));
    fireEvent.click(await screen.findByRole("button", { name: "Keep Mia" }));
    expect(screen.queryByRole("alert")).toBeNull();
    expect((await mia.me()).profile.name).toBe("Mia");
    // a new profile instead: asked again, and this time going on
    fireEvent.click(
      screen.getByRole("button", { name: "New here? Create a profile" }),
    );
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "Lea" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create my profile" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete it" }));
    await waitFor(() => expect(r.path()).toBe("/"));
    expect((await mia.me()).profile.name).toBe("Lea");
    expect(world.profiles.has("p-mia")).toBe(false);
  });

  it("a code's link opened in a tab already on /pair takes that code", async () => {
    const world = new FakeWorld();
    const fresh = world.device();
    const r = renderApp("/pair", fresh);
    fireEvent.click(
      await screen.findByRole("button", { name: "New here? Create a profile" }),
    );
    const { code } = await world.device("p-mia").pair();
    act(() => r.loc.navigate(`/pair/${code}`));
    expect(await screen.findByLabelText("Your code")).toHaveValue(code);
  });

  it("the admin's only device is refused, with the reason", async () => {
    const world = new FakeWorld({ seed: false });
    const admin = world.device();
    await admin.setup({ token: "fake-setup", name: "Jan", avatar: "vinyl" });
    renderApp("/pair", admin);
    fireEvent.click(
      await screen.findByRole("button", { name: "New here? Create a profile" }),
    );
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "Lea" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create my profile" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "admin profile's only device",
    );
  });

  it("back from the code to a new profile; a refused name is shown", async () => {
    const world = new FakeWorld();
    const api = world.device();
    api.createProfile = () =>
      Promise.reject(
        new ApiError(400, "invalid_name", "a name is 1 to 24 characters"),
      );
    renderApp("/me", api);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Have a profile on another device? Use its code",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "New here? Create a profile" }),
    );
    fireEvent.change(screen.getByLabelText("Your name"), {
      target: { value: "x" },
    });
    fireEvent.keyDown(screen.getByLabelText("Your name"), { key: "Enter" });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "a name is 1 to 24 characters",
    );
  });

  it("a fresh install shows the setup card instead of a code field", async () => {
    renderApp("/me", new FakeWorld({ seed: false }).device());
    expect(
      await screen.findByText("Reworm isn't set up yet"),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Your code")).toBeNull();
  });
});
