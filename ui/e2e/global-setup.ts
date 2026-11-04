// The harness: builds the UI and the real Go binary, writes the fixture clips, starts the server on a free port
// with a fresh data dir and the BACKEND_DEV_* flags, and reads the one-time setup link from its log. The tests
// find it all through E2E_* variables. Teardown stops the server and removes the dir (E2E_KEEP=1 keeps it).
import { execFileSync, spawn } from "node:child_process";
import { createWriteStream } from "node:fs";
import fs from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
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

  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    BACKEND_ADDR: `127.0.0.1:${port}`,
    BACKEND_DATA_DIR: path.join(dir, "data"),
    BACKEND_DEV_PREVIEWS: clips,
    BACKEND_DEV_SONGS: path.join(here, "fixtures/songs.jsonl"),
    BACKEND_LOG_LEVEL: "info",
  };
  delete env.PUBLIC_FQDN;
  const server = spawn(bin, [], { env, stdio: ["ignore", "pipe", "pipe"] });
  const log = createWriteStream(path.join(dir, "server.log"));
  const seen: { setup: string; exited: number | null } = {
    setup: "",
    exited: null,
  };
  server.on("exit", (code) => (seen.exited = code ?? -1));
  for (const out of [server.stdout, server.stderr]) {
    out.on("data", (chunk: Buffer) => {
      log.write(chunk);
      // the first device's one-time link, as the log prints it: url=/setup/<token>
      const m = /url=(\S*\/setup\/[A-Za-z0-9_-]+)/.exec(chunk.toString());
      if (m) seen.setup = m[1];
    });
  }

  const deadline = Date.now() + 30_000;
  for (;;) {
    if (seen.exited != null)
      throw new Error(`server exited (${seen.exited}), see ${dir}/server.log`);
    if (Date.now() > deadline)
      throw new Error(`server did not come up, see ${dir}/server.log`);
    const up = await fetch(`${base}/healthz`).then(
      (r) => r.ok,
      () => false,
    );
    if (up && seen.setup) break;
    await sleep(100);
  }

  process.env.E2E_BASE_URL = base;
  process.env.E2E_DIR = dir;
  process.env.E2E_SETUP_PATH = seen.setup;
  console.log(`e2e: server ${base}, data in ${dir}`);

  return async () => {
    server.kill("SIGTERM");
    for (let i = 0; i < 100 && seen.exited == null; i++) await sleep(100);
    if (seen.exited == null) server.kill("SIGKILL");
    log.end();
    if (!process.env.E2E_KEEP) await fs.rm(dir, { recursive: true });
  };
}
