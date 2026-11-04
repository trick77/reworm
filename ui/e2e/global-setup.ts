// The harness: builds the UI and the real Go binary, writes the fixture clips, starts the server on a free port
// with a fresh data dir in dev mode (BACKEND_DEV=1 + BACKEND_DEV_*), and reads the one-time setup link from its
// log. The tests find it all through E2E_* variables. Teardown, a failed start and the runner's exit stop the
// server; teardown and a failed start remove the dir (E2E_KEEP=1 keeps it).
import { execFileSync, spawn } from "node:child_process";
import { createWriteStream } from "node:fs";
import fs from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { SONGS } from "./songs";

const here = path.dirname(fileURLToPath(import.meta.url));
const ui = path.resolve(here, "..");
const backend = path.resolve(ui, "../backend");

// wav is secs seconds of a sine tone, 8 kHz 8-bit mono PCM: small, and every browser engine plays it.
function wav(hz: number, secs: number): Buffer {
  const rate = 8000;
  const n = rate * secs;
  const b = Buffer.alloc(44 + n);
  b.write("RIFF", 0);
  b.writeUInt32LE(36 + n, 4);
  b.write("WAVEfmt ", 8);
  b.writeUInt32LE(16, 16);
  b.writeUInt16LE(1, 20); // PCM
  b.writeUInt16LE(1, 22); // mono
  b.writeUInt32LE(rate, 24);
  b.writeUInt32LE(rate, 28); // bytes per second
  b.writeUInt16LE(1, 32); // block align
  b.writeUInt16LE(8, 34); // bits per sample
  b.write("data", 36);
  b.writeUInt32LE(n, 40);
  for (let i = 0; i < n; i++)
    b[44 + i] = Math.round(128 + 40 * Math.sin((2 * Math.PI * hz * i) / rate));
  return b;
}

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const s = net.createServer();
    s.once("error", reject);
    s.listen(0, "127.0.0.1", () => {
      const port = (s.address() as net.AddressInfo).port;
      s.close(() => resolve(port));
    });
  });
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export default async function globalSetup() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "reworm-e2e-"));
  const cleanup: (() => Promise<void> | void)[] = [];
  // everything started is undone: by teardown, by a failed start, and on any exit of the runner
  const undo = async () => {
    for (const f of cleanup.splice(0).reverse()) await f();
    if (!process.env.E2E_KEEP)
      await fs.rm(dir, { recursive: true, force: true });
  };
  try {
    const s = await start(dir, cleanup);
    process.env.E2E_BASE_URL = s.base;
    process.env.E2E_DIR = dir;
    process.env.E2E_SETUP_PATH = s.setup;
    console.log(`e2e: server ${s.base}, data in ${dir}`);
    return undo;
  } catch (e) {
    await undo();
    throw e;
  }
}

async function start(
  dir: string,
  cleanup: (() => Promise<void> | void)[],
): Promise<{ base: string; setup: string }> {
  const clips = path.join(dir, "clips");
  await fs.mkdir(clips);
  await Promise.all(
    SONGS.map((s, i) =>
      fs.writeFile(path.join(clips, `${s.id}.wav`), wav(330 + 40 * i, 20)),
    ),
  );

  if (!process.env.E2E_SKIP_BUILD) {
    // as make ui-build: vite does not empty the embed dir, old hashed bundles would be embedded too
    const dist = path.join(backend, "web/dist");
    for (const f of await fs.readdir(dist))
      if (f !== ".gitkeep")
        await fs.rm(path.join(dist, f), { recursive: true });
    execFileSync("npm", ["run", "build"], { cwd: ui, stdio: "inherit" });
  }
  const bin = path.join(dir, "reworm");
  execFileSync("go", ["build", "-o", bin, "./cmd/reworm"], {
    cwd: backend,
    stdio: "inherit",
  });

  // kit's Serve binds BACKEND_ADDR itself and logs no bound port, so a :0 can't be read back: a free port is
  // picked here, and a start that lost it to someone else meanwhile is tried again on another one.
  for (let attempt = 1; ; attempt++) {
    try {
      return await launch(dir, bin, clips, await freePort(), cleanup);
    } catch (e) {
      if (!(e instanceof PortTaken) || attempt === 5) throw e;
      await cleanup.pop()?.();
    }
  }
}

// PortTaken: the port was bound by someone else between freePort and the server's own bind.
class PortTaken extends Error {}

async function launch(
  dir: string,
  bin: string,
  clips: string,
  port: number,
  cleanup: (() => Promise<void> | void)[],
): Promise<{ base: string; setup: string }> {
  const base = `http://127.0.0.1:${port}`;
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    BACKEND_ADDR: `127.0.0.1:${port}`,
    BACKEND_DATA_DIR: path.join(dir, "data"),
    BACKEND_DEV: "1",
    BACKEND_DEV_PREVIEWS: clips,
    BACKEND_DEV_SONGS: path.join(here, "fixtures/songs.jsonl"),
    BACKEND_LOG_LEVEL: "info",
  };
  delete env.PUBLIC_FQDN;
  const server = spawn(bin, [], { env, stdio: ["ignore", "pipe", "pipe"] });
  const seen: { setup: string; exited: number | null; tail: string[] } = {
    setup: "",
    exited: null,
    tail: [],
  };
  server.on("exit", (code) => (seen.exited = code ?? -1));
  // the runner dying any way (an error, ^C) must not leave the server behind
  const kill = () => {
    if (seen.exited == null) server.kill("SIGKILL");
  };
  process.on("exit", kill);
  const log = createWriteStream(path.join(dir, "server.log"), { flags: "a" });
  cleanup.push(async () => {
    process.off("exit", kill);
    if (seen.exited == null) server.kill("SIGTERM");
    for (let i = 0; i < 100 && seen.exited == null; i++) await sleep(100);
    kill();
    await new Promise<void>((r) => log.end(r));
  });
  for (const out of [server.stdout, server.stderr]) {
    // whole lines: the setup link may straddle two chunks
    readline.createInterface({ input: out }).on("line", (line) => {
      log.write(line + "\n");
      seen.tail = [...seen.tail.slice(-19), line];
      // the first device's one-time link, as the log prints it: url=/setup/<token>
      const m = /url=(\S*\/setup\/[A-Za-z0-9_-]+)/.exec(line);
      if (m) seen.setup = m[1];
    });
  }

  const fail = (why: string) => {
    const msg = `server ${why}; its log ends:\n${seen.tail.join("\n")}`;
    return seen.tail.some((l) => /address already in use/i.test(l))
      ? new PortTaken(msg)
      : new Error(msg);
  };
  const deadline = Date.now() + 30_000;
  for (;;) {
    if (seen.exited != null) throw fail(`exited (${seen.exited})`);
    if (Date.now() > deadline) throw fail("did not come up in 30 s");
    const up = await fetch(`${base}/healthz`).then(
      (r) => r.ok,
      () => false,
    );
    if (up && seen.setup) {
      // a failed bind ends the binary at once: still running a moment later, the answer was its own
      await sleep(300);
      if (seen.exited != null) throw fail(`exited (${seen.exited})`);
      return { base, setup: seen.setup };
    }
    await sleep(100);
  }
}
