import assert from "node:assert/strict";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { createServer } from "node:http";
import { probe, workspaceContext } from "./session-context.mjs";

const repository = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const script = join(repository, ".codex/hooks/codex-hooks.mjs");

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), "codex-hooks-test-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const git = (...args) => {
    const result = spawnSync("git", args, {
      cwd: root,
      encoding: "utf8",
      timeout: 10000,
    });
    assert.equal(result.status, 0, result.stderr);
  };
  const write = (file, content) => {
    mkdirSync(dirname(join(root, file)), { recursive: true });
    writeFileSync(join(root, file), content);
  };
  git("init", "-b", "main");
  git("config", "user.name", "Hook test");
  git("config", "user.email", "hooks@example.invalid");
  write(".gitignore", ".dev/\nnode_modules/\n");
  git("add", ".");
  git("commit", "-m", "fixture");
  git("update-ref", "refs/remotes/origin/main", "HEAD");
  const invoke = (mode, extra = {}) => {
    const result = spawnSync(process.execPath, [script, mode], {
      input: JSON.stringify({
        cwd: root,
        session_id: "test",
        tool_use_id: "call",
        ...extra,
      }),
      encoding: "utf8",
      timeout: 30000,
    });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim() ? JSON.parse(result.stdout) : {};
  };
  return { root, git, write, invoke };
}

test("formats only files changed by the paired call, including new files and spaced paths", (t) => {
  const { root, write, invoke } = fixture(t);
  write("backend/untouched.go", "package example\nfunc Old( ){ }\n");
  write("backend/changed.go", "package example\n");
  write("frontend/.prettierrc.json", '{"singleQuote":true}');
  write("frontend/.prettierignore", "ignored.ts\n");
  symlinkSync(
    join(repository, "frontend/node_modules"),
    join(root, "frontend/node_modules"),
  );
  invoke("snapshot");
  write("backend/changed.go", "package example\nfunc New( ){ }\n");
  write(
    "backend/internal/storage/postgres/sqlc/generated.go",
    "package example\nfunc Generated( ){ }\n",
  );
  write("frontend/new file.ts", 'const value="hello"');
  write("frontend/ignored.ts", 'const value="hello"');
  const result = invoke("format");
  assert.match(result.hookSpecificOutput.additionalContext, /new file.ts/);
  assert.equal(
    readFileSync(join(root, "backend/changed.go"), "utf8"),
    "package example\n\nfunc New() {}\n",
  );
  assert.equal(
    readFileSync(join(root, "backend/untouched.go"), "utf8"),
    "package example\nfunc Old( ){ }\n",
  );
  assert.equal(
    readFileSync(join(root, "frontend/new file.ts"), "utf8"),
    "const value = 'hello';\n",
  );
  assert.equal(
    readFileSync(join(root, "frontend/ignored.ts"), "utf8"),
    'const value="hello"',
  );
  assert.equal(
    readFileSync(
      join(root, "backend/internal/storage/postgres/sqlc/generated.go"),
      "utf8",
    ),
    "package example\nfunc Generated( ){ }\n",
  );
  assert.deepEqual(invoke("format"), {});
});

test("isolates snapshots by tool id and skips symlinks and Plan mode", (t) => {
  const { root, write, invoke } = fixture(t);
  write("backend/file.go", "package example\n");
  invoke("snapshot", { tool_use_id: "first" });
  write("backend/file.go", "package example\nfunc New( ){ }\n");
  assert.deepEqual(invoke("format", { tool_use_id: "other" }), {});
  invoke("format", { permission_mode: "plan" });
  assert.match(readFileSync(join(root, "backend/file.go"), "utf8"), /New\( \)/);
  write("outside.go", "package example\nfunc Outside( ){ }\n");
  symlinkSync(join(root, "outside.go"), join(root, "backend/link.go"));
  invoke("format", { tool_use_id: "first" });
  assert.match(readFileSync(join(root, "outside.go"), "utf8"), /Outside\( \)/);
});

test("OpenAPI skips unrelated work, checks committed work, and limits failure continuations", (t) => {
  const { root, git, write, invoke } = fixture(t);
  write("frontend/file.ts", "export {};");
  assert.deepEqual(invoke("openapi"), {});
  write(
    ".agents/skills/openapi-contract/scripts/check-openapi.sh",
    "#!/bin/sh\necho contract-mismatch\nexit 1\n",
  );
  write("backend/file.go", "package example\n");
  assert.match(invoke("openapi").reason, /contract-mismatch/);
  const retry = invoke("openapi", { stop_hook_active: true });
  assert.equal(retry.decision, undefined);
  assert.match(retry.systemMessage, /contract-mismatch/);
  assert.deepEqual(invoke("openapi", { permission_mode: "plan" }), {});
  git("add", ".");
  git("commit", "-m", "backend changes");
  assert.equal(invoke("openapi").decision, "block");
  write(
    ".agents/skills/openapi-contract/scripts/check-openapi.sh",
    "#!/bin/sh\nexit 0\n",
  );
  assert.deepEqual(invoke("openapi"), {});
});

test("formatter reports parse errors instead of claiming success", (t) => {
  const { write, invoke } = fixture(t);
  invoke("snapshot");
  write("backend/broken.go", "package broken\nfunc {");
  assert.match(invoke("format").systemMessage, /gofmt failed/);
});

test("session context reports clean, dirty and detached repositories from nested directories", async (t) => {
  const { root, git, write } = fixture(t);
  const context = await workspaceContext(root);
  assert.match(
    context,
    /API readiness http:\/\/localhost:8080\/api\/v1\/health\/ready:/,
  );
  assert.doesNotMatch(context, /\/auth\/me/);
  assert.match(context, /Branch: "main"\. Uncommitted changes: none/);
  write("nested/new.txt", "demo");
  assert.match(
    await workspaceContext(join(root, "nested")),
    /Uncommitted changes: yes/,
  );
  git("checkout", "--detach");
  assert.match(await workspaceContext(root), /detached HEAD/);
  assert.match(await workspaceContext(tmpdir()), /Git context unavailable/);
});

test("HTTP probes distinguish responses, refused connections and timeouts without reading bodies", async (t) => {
  const server = createServer((req, res) => {
    if (req.url === "/hang") return;
    assert.equal(req.method, "HEAD");
    res.writeHead(req.url === "/auth" ? 401 : 200);
    res.end();
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => {
    server.closeAllConnections();
    server.close();
  });
  const url = `http://127.0.0.1:${server.address().port}`;
  assert.equal(await probe(url), "HTTP 200");
  assert.equal(await probe(`${url}/auth`), "HTTP 401");
  assert.equal(await probe(`${url}/hang`, 50), "unavailable (TIMEOUT)");
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
  assert.equal(await probe(url), "unavailable (ECONNREFUSED)");
});
