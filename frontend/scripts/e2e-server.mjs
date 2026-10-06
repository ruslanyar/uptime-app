// Real Go API + disposable PostgreSQL database + two independent browser origins.
import { spawn } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, join } from "node:path";
import { randomBytes } from "node:crypto";
import https from "node:https";
import http from "node:http";
import net from "node:net";
const backend = resolve("../backend");
const temporary = await mkdtemp(join(tmpdir(), "uptime-frontend-e2e-"));
const database = `frontend_e2e_${randomBytes(8).toString("hex")}`;
const children = [];
const commands = new Set();
const servers = [];
let created = false;
let postgresRequested = false;
let closing = false;
function command(cmd, args, options = {}) {
  return new Promise((done, fail) => {
    const child = spawn(cmd, args, { stdio: "inherit", ...options });
    commands.add(child);
    child.once("error", (error) => { commands.delete(child); fail(error); });
    child.once("exit", (code) => {
      commands.delete(child);
      if (code === 0) done(); else fail(new Error(`${cmd} exited ${code}`));
    });
  });
}
function start(cmd, args, options) {
  const child = spawn(cmd, args, { stdio: "inherit", ...options });
  children.push(child);
  child.on("error", () => void close(1));
  child.on("exit", () => { if (!closing) void close(1); });
}
async function sql(statement) {
  await command("docker", ["compose", "-f", "compose.yaml", "--profile", "test", "exec", "-T", "postgres-test", "psql", "-U", "uptime_test", "-d", "uptime_test", "-v", "ON_ERROR_STOP=1", "-c", statement], { cwd: backend, timeout: 10000 });
}
async function close(code = 0) {
  if (closing) return;
  closing = true;
  for (const server of servers) server.close();
  await Promise.all([...children, ...commands].map((child) => new Promise((done) => {
    if (!child.pid || child.exitCode !== null || child.signalCode !== null) { done(); return; }
    const timeout = setTimeout(() => child.kill("SIGKILL"), 4000); timeout.unref();
    child.once("exit", () => { clearTimeout(timeout); done(); });
    child.kill("SIGTERM");
  })));
  // Each cleanup step must run even if the preceding one failed.
  const cleanup = async (task) => {
    try { await task(); }
    catch (error) { console.error(`E2E cleanup failed: ${error.message}`); code = 1; }
  };
  if (created) await cleanup(() => sql(`DROP DATABASE "${database}" WITH (FORCE)`));
  if (postgresRequested) await cleanup(() => command("docker", [
    "compose", "-f", "compose.yaml", "--profile", "test", "rm", "--stop", "--force", "postgres-test",
  ], { cwd: backend, timeout: 20000 }));
  await cleanup(() => rm(temporary, { recursive: true, force: true }));
  process.exit(code);
}
process.on("SIGTERM", () => void close());
process.on("SIGINT", () => void close());
try {
  postgresRequested = true;
  await command("docker", ["compose", "-f", "compose.yaml", "--profile", "test", "up", "-d", "--wait", "--wait-timeout", "60", "postgres-test"], { cwd: backend });
  await sql(`CREATE DATABASE "${database}"`); created = true;
  const env = { ...process.env, DATABASE_URL: `postgres://uptime_test:local-test-only@localhost:55432/${database}?sslmode=disable`, JWT_SECRET: process.env.FRONTEND_E2E_JWT_SECRET ?? randomBytes(32).toString("hex"), JWT_ISSUER: "frontend-e2e", JWT_AUDIENCE: "frontend-e2e" };
  await command("go", ["run", "./cmd/migrate", "up"], { cwd: backend, env });
  const binary = join(temporary, "api");
  await command("go", ["build", "-o", binary, "./cmd/api"], { cwd: backend });
  start(binary, [], { env: { ...env, HTTP_ADDR: "127.0.0.1:18080", ALLOWED_ORIGINS: "http://localhost:13000", COOKIE_SECURE: "false", COOKIE_SAME_SITE: "lax" } });
  start(binary, [], { env: { ...env, HTTP_ADDR: "127.0.0.1:18081", ALLOWED_ORIGINS: "https://frontend.auth-client.test:13443", COOKIE_SECURE: "true", COOKIE_SAME_SITE: "none" } });
  const next = resolve("node_modules/next/dist/bin/next");
  start(process.execPath, [next, "dev", "-p", "13000"], { env: { ...process.env, NEXT_PUBLIC_API_URL: "http://localhost:18080", E2E_DIST_DIR: ".next-e2e-http" } });
  start(process.execPath, [next, "dev", "-p", "13001"], { env: { ...process.env, NEXT_PUBLIC_API_URL: "https://api.auth-service.test:18443", E2E_DIST_DIR: ".next-e2e-https" } });
  await command("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", join(temporary, "key.pem"), "-out", join(temporary, "cert.pem"), "-days", "1", "-subj", "/CN=frontend-e2e"], { stdio: "ignore" });
  const tls = { key: await readFile(join(temporary, "key.pem")), cert: await readFile(join(temporary, "cert.pem")) };
  for (const [port, target] of [[13443, 13001], [18443, 18081]]) {
    const server = https.createServer(tls, (request, response) => {
      const upstream = http.request({ hostname: "127.0.0.1", port: target, path: request.url, method: request.method, headers: request.headers }, (result) => { response.writeHead(result.statusCode, result.headers); result.pipe(response); });
      upstream.on("error", () => { response.writeHead(502); response.end(); });
      request.pipe(upstream);
    });
    server.on("upgrade", (request, socket, head) => {
      const upstream = net.connect(target, "127.0.0.1", () => {
        const headers = Object.entries(request.headers).map(([key, value]) => `${key}: ${value}`).join("\r\n");
        upstream.write(`${request.method} ${request.url} HTTP/1.1\r\n${headers}\r\n\r\n`);
        if (head.length) upstream.write(head);
        socket.pipe(upstream); upstream.pipe(socket);
      });
      upstream.on("error", () => socket.destroy());
      socket.on("error", () => upstream.destroy());
      socket.on("close", () => upstream.destroy());
    });
    server.listen(port, "127.0.0.1"); servers.push(server);
  }
  // Readiness checks both Next instances and APIs; this endpoint never serves app data.
  const ready = http.createServer(async (_request, response) => {
    try {
      for (const port of [13000, 13001, 18080, 18081]) {
        const result = await fetch(`http://localhost:${port}${port < 18000 ? "/login" : "/api/v1/auth/me"}`);
        if (result.status !== (port < 18000 ? 200 : 401)) throw new Error();
      }
      response.end("ready");
    } catch { response.writeHead(503); response.end(); }
  });
  ready.listen(13999, "127.0.0.1"); servers.push(ready);
} catch (error) { console.error(error.message); await close(1); }
