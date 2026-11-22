import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Router } from "wouter";
import { BASE } from "./base";
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
  const q = new URLSearchParams(window.location.search);
  if (q.get("fake") === "0") sessionStorage.removeItem("reworm.fake");
  if (q.get("fake") === "1") sessionStorage.setItem("reworm.fake", "1");
  if (sessionStorage.getItem("reworm.fake") !== "1") return undefined;
  const { FakeWorld } = await import("./test/fakeApi");
  // ?fresh=1: a fresh install, no profile yet (its setup link is /setup/fake-setup), kept for the tab too
  if (q.get("fresh") === "0") sessionStorage.removeItem("reworm.fresh");
  if (q.get("fresh") === "1") sessionStorage.setItem("reworm.fresh", "1");
  const fresh = sessionStorage.getItem("reworm.fresh") === "1";
  const world = new FakeWorld({ timers: true, latency: 120, seed: !fresh });
  // ?guest=1: an unpaired phone (a party guest), else Jan's paired device
  const guest = fresh || q.get("guest") === "1";
  const api = world.device(guest ? null : "p-jan", "This browser");
  Object.assign(window, { __fake: world, __fakeApi: api });
  return api;
}

void fakeApi().then((api) =>
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      {/* every route and go("/…") is app-relative: the router adds the base path */}
      <Router base={BASE}>
        <App api={api} />
      </Router>
    </StrictMode>,
  ),
);
