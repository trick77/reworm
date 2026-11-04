// renderApp renders the whole app at a path on a fake device, with an in-memory location.
import { render } from "@testing-library/react";
import { Router } from "wouter";
import { memoryLocation } from "wouter/memory-location";
import App from "../App";
import type { FakeApi } from "./fakeApi";

export function renderApp(path: string, api: FakeApi) {
  const loc = memoryLocation({ path, record: true });
  const r = render(
    <Router hook={loc.hook} searchHook={loc.searchHook}>
      <App api={api} dev={false} />
    </Router>,
  );
  return { ...r, loc, path: () => loc.history.at(-1) };
}
