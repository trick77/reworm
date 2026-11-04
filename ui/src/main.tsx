import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles/tokens.css";
import "./styles/components.css";
import "./styles/screens.css";
import "./styles/layout.css";
import App from "./App";
import type { Api } from "./api/client";

// ?fake=1 on the dev server: the app runs on the in-memory fake server (src/test/fakeApi.ts), kept for the
// tab; window.__fake is its world, to pose any screen from the console.
async function fakeApi(): Promise<Api | undefined> {
  if (!import.meta.env.DEV) return undefined;
  const q = new URLSearchParams(window.location.search).get("fake");
  if (q === "0") sessionStorage.removeItem("reworm.fake");
  if (q === "1") sessionStorage.setItem("reworm.fake", "1");
  if (sessionStorage.getItem("reworm.fake") !== "1") return undefined;
  const { FakeWorld } = await import("./test/fakeApi");
  const world = new FakeWorld({ timers: true, latency: 120 });
  // ?guest=1: an unpaired phone (a party guest), else Jan's paired device
  const guest =
    new URLSearchParams(window.location.search).get("guest") === "1";
  const api = world.device(guest ? null : "p-jan", "This browser");
  Object.assign(window, { __fake: world, __fakeApi: api });
  return api;
}

void fakeApi().then((api) =>
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App api={api} />
    </StrictMode>,
  ),
);
