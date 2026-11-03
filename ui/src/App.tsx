import { useEffect, useState } from "react";
import { fetchVersion } from "./api";
import { WormLogo } from "./WormLogo";
import "./App.css";

export default function App() {
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
