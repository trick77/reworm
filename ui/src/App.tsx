import { lazy, Suspense } from "react";
import { Route, Switch, useLocation } from "wouter";
import type { Api } from "./api/client";
import { ApiProvider, useApi } from "./api/context";
import { Button } from "./components/Controls";
import { Screen } from "./screens/common";
import { Home } from "./screens/Home";
import { Join } from "./screens/party/Join";
import { Host, Party, Room } from "./screens/party/Party";
import { MePage, Pair, Setup, Who } from "./screens/Profile";
import { Settings } from "./screens/Settings";
import { Training } from "./screens/train/Training";
import { TrainSetup } from "./screens/train/TrainSetup";

// The component gallery is for the dev server only; a production build leaves it out.
const Gallery = import.meta.env.DEV
  ? lazy(() => import("./gallery/Gallery"))
  : null;

function NotFound() {
  const [, go] = useLocation();
  return (
    <Screen>
      <div className="empty">
        <div className="h-title">Nothing here</div>
        <div className="sub" style={{ margin: "6px 0 18px" }}>
          This page doesn't exist.
        </div>
        <Button small style={{ margin: "0 auto" }} onClick={() => go("/")}>
          Home
        </Button>
      </div>
    </Screen>
  );
}

function Routes({ dev }: { dev: boolean }) {
  return (
    <Switch>
      <Route path="/" component={Home} />
      <Route path="/party" component={Party} />
      <Route path="/host" component={Host} />
      <Route path="/room/:code">
        {(p) => <Room key={p.code} code={p.code.toUpperCase()} />}
      </Route>
      <Route path="/join/:code?">
        {(p) => <Join key={p.code ?? ""} code={p.code} />}
      </Route>
      <Route path="/pair/:code?">{(p) => <Pair code={p.code} />}</Route>
      <Route path="/setup/:token?">{(p) => <Setup token={p.token} />}</Route>
      <Route path="/who" component={Who} />
      <Route path="/me" component={MePage} />
      <Route path="/train" component={TrainSetup} />
      <Route path="/train/:id">
        {(p) => <Training key={p.id} id={p.id} />}
      </Route>
      <Route path="/settings" component={Settings} />
      {dev && Gallery && (
        <Route path="/gallery">
          <Suspense fallback={null}>
            <Gallery />
          </Suspense>
        </Route>
      )}
      <Route component={NotFound} />
    </Switch>
  );
}

export default function App({
  api,
  dev = import.meta.env.DEV,
}: {
  api?: Api;
  dev?: boolean;
}) {
  const fallback = useApi();
  return (
    <ApiProvider api={api ?? fallback}>
      <Routes dev={dev} />
    </ApiProvider>
  );
}
