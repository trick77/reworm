import { act, render, screen } from "@testing-library/react";
import App from "./App";

function stubFetch(response: Promise<Response>) {
  const fetchMock = vi.fn(() => response);
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("App", () => {
  it("shows the wordmark and the server's version", async () => {
    const fetchMock = stubFetch(
      Promise.resolve(Response.json({ version: "0.1.0" })),
    );
    render(<App />);
    expect(screen.getByText("worm")).toBeInTheDocument();
    expect(await screen.findByText("0.1.0")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith("/api/version", expect.anything());
  });

  it("renders without a version when the server is unreachable", async () => {
    const fetchMock = stubFetch(Promise.reject(new TypeError("offline")));
    const { container } = render(<App />);
    expect(fetchMock).toHaveBeenCalled();
    // A macrotask runs only after every pending microtask: the rejection has
    // gone through fetchVersion and App's handler by then.
    await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
    expect(screen.getByText("worm")).toBeInTheDocument();
    expect(container.querySelector(".version")).toBeNull();
  });

  it("shows the gallery on #gallery in dev, the landing otherwise", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.reject(new TypeError("offline"))),
    );
    window.location.hash = "#gallery";
    const { unmount } = render(<App dev />);
    expect(await screen.findByText("component gallery")).toBeInTheDocument();
    act(() => {
      window.location.hash = "";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(screen.queryByText("component gallery")).toBeNull();
    unmount();

    window.location.hash = "#gallery";
    render(<App dev={false} />);
    expect(screen.queryByText("component gallery")).toBeNull();
    expect(screen.getByText("worm")).toBeInTheDocument();
    window.location.hash = "";
  });
});
