import { readFileSync } from "node:fs";
import { request } from "node:http";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

function git(cwd, args) {
  const result = spawnSync("git", args, {
    cwd,
    encoding: "utf8",
    timeout: 3000,
    maxBuffer: 4 * 1024 * 1024,
  });
  if (result.error || result.status !== 0)
    throw new Error("Git context unavailable");
  return result.stdout.trim();
}

/** Report HTTP reachability, including error statuses, without claiming application health. */
export function probe(url, timeout = 2000) {
  return new Promise((resolve) => {
    let timer;
    const req = request(url, { method: "HEAD", agent: false }, (response) => {
      clearTimeout(timer);
      response.destroy();
      resolve(`HTTP ${response.statusCode}`);
    });
    req.on("error", (error) => {
      clearTimeout(timer);
      resolve(`unavailable (${error.code ?? "request failed"})`);
    });
    // A wall-clock timer also bounds connection establishment, not just socket inactivity.
    timer = setTimeout(
      () =>
        req.destroy(Object.assign(new Error("timeout"), { code: "TIMEOUT" })),
      timeout,
    );
    req.end();
  });
}

/** Return concise, read-only Git and local HTTP observations, also usable in Plan mode. */
export async function workspaceContext(cwd) {
  let repository;
  try {
    const root = git(cwd, ["rev-parse", "--show-toplevel"]);
    const branch = git(root, ["rev-parse", "--abbrev-ref", "HEAD"]);
    const label =
      branch === "HEAD"
        ? `detached HEAD (${git(root, ["rev-parse", "--short", "HEAD"])})`
        : branch;
    const dirty = git(root, [
      "status",
      "--porcelain",
      "--untracked-files=normal",
    ]);
    repository = `Branch: ${JSON.stringify(label)}. Uncommitted changes: ${dirty ? "yes (staged, unstaged or untracked)" : "none"}.`;
  } catch {
    repository = "Git context unavailable (not a repository or Git failed).";
  }
  const targets = [
    ["Frontend", "http://localhost:3000/"],
    ["API readiness", "http://localhost:8080/api/v1/health/ready"],
  ];
  const observations = await Promise.all(
    targets.map(async ([name, url]) => `${name} ${url}: ${await probe(url)}.`),
  );
  return [
    repository,
    ...observations,
    "Frontend HTTP responses indicate reachability only. API readiness: HTTP 200 means ready, HTTP 503 means not ready; other statuses do not confirm readiness. Addresses are the canonical local defaults; custom ports are not probed.",
  ].join("\n");
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  const event = JSON.parse(readFileSync(0, "utf8"));
  console.log(
    JSON.stringify({
      hookSpecificOutput: {
        hookEventName: "SessionStart",
        additionalContext: await workspaceContext(event.cwd),
      },
    }),
  );
}
