import { lazy, Suspense, useEffect, useState } from "react";
import { fetchVersion } from "./api";
import { WormLogo } from "./WormLogo";
import "./App.css";

// The component gallery is for the dev server only; a production build leaves it out.
const Gallery = lazy(() => import("./gallery/Gallery"));

function useHash() {
  const [hash, setHash] = useState(() => window.location.hash);
  useEffect(() => {
    const on = () => setHash(window.location.hash);
    window.addEventListener("hashchange", on);
    return () => window.removeEventListener("hashchange", on);
  }, []);
  return hash;
}

export default function App({ dev = import.meta.env.DEV }: { dev?: boolean }) {
  const hash = useHash();
  if (dev && hash === "#gallery") {
    return (
      <Suspense fallback={null}>
        <Gallery />
      </Suspense>
    );
  }
  return <Landing />;
}

function Landing() {
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    fetchVersion(ctrl.signal).then(setVersion, () => {
      // An unreachable server leaves the version out; the shell still renders.
    });
    return () => ctrl.abort();
  }, []);

  return (
    <main className="shell">
      <div className="wordmark">
        <div className="seal">
          <WormLogo />
        </div>
        <div className="brand">
          Re<span>worm</span>
        </div>
      </div>
      {version && <div className="version num">{version}</div>}
    </main>
  );
}
