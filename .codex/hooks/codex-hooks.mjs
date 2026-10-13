import { createHash } from "node:crypto";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  realpathSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { dirname, extname, join, relative } from "node:path";
import { spawnSync } from "node:child_process";

const event = JSON.parse(readFileSync(0, "utf8"));
const mode = process.argv[2];

function run(command, args, cwd, timeout = 10000) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    timeout,
    maxBuffer: 8 * 1024 * 1024,
  });
  if (result.error || result.status !== 0) {
    throw new Error(
      `${command} failed: ${result.error?.message ?? `${result.stdout}\n${result.stderr}`}`,
    );
  }
  return result.stdout;
}

function hash(value) {
  return createHash("sha256").update(value).digest("hex");
}

function files(root) {
  return [
    ...new Set(
      run(
        "git",
        ["ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        root,
      )
        .split("\0")
        .filter(Boolean),
    ),
  ];
}

function safeFile(root, file) {
  const path = join(root, file);
  if (!existsSync(path) || !lstatSync(path).isFile()) return false;
  return !relative(root, realpathSync(path)).startsWith("..");
}

function formatCandidate(file) {
  if (file.startsWith("backend/") && file.endsWith(".go")) {
    return !file.startsWith("backend/internal/storage/postgres/sqlc/");
  }
  return (
    file.startsWith("frontend/") &&
    /\.(tsx?|jsx?|mjs|cjs|json|css|md|ya?ml)$/.test(file) &&
    !["package-lock.json", "next-env.d.ts"].includes(file.split("/").at(-1))
  );
}

function snapshot(root) {
  return Object.fromEntries(
    files(root)
      .filter(formatCandidate)
      .filter((file) => safeFile(root, file))
      .map((file) => [file, hash(readFileSync(join(root, file)))]),
  );
}

try {
  if (event.permission_mode === "plan") {
    if (mode === "openapi") console.log("{}");
    process.exit(0);
  }
  const root = run("git", ["rev-parse", "--show-toplevel"], event.cwd).trim();
  if (mode !== "openapi" && !event.tool_use_id)
    throw new Error("Missing tool_use_id for formatter snapshot");
  // Pair snapshots by tool call, so existing user edits are never formatted merely for being dirty.
  const state = join(
    root,
    ".dev",
    "codex-hooks",
    `${hash(`${event.session_id}:${event.tool_use_id}`)}.json`,
  );
  if (mode === "snapshot") {
    mkdirSync(dirname(state), { recursive: true });
    writeFileSync(state, JSON.stringify(snapshot(root)), { mode: 0o600 });
  } else if (mode === "format") {
    if (!existsSync(state)) process.exit(0);
    const before = JSON.parse(readFileSync(state, "utf8"));
    unlinkSync(state);
    const after = snapshot(root);
    const changed = Object.keys(after).filter(
      (file) => before[file] !== after[file],
    );
    const go = changed.filter((file) => extname(file) === ".go");
    const frontend = changed.filter((file) => file.startsWith("frontend/"));
    if (go.length) run("gofmt", ["-w", ...go], root, 20000);
    if (frontend.length) {
      // Use the installed, pinned formatter and the frontend's ignore/config rules; never download via npx.
      run(
        process.execPath,
        [
          "node_modules/prettier/bin/prettier.cjs",
          "--write",
          "--ignore-unknown",
          "--ignore-path",
          ".prettierignore",
          "--",
          ...frontend.map((file) => file.slice("frontend/".length)),
        ],
        join(root, "frontend"),
        20000,
      );
    }
    if (changed.length)
      console.log(
        JSON.stringify({
          hookSpecificOutput: {
            hookEventName: "PostToolUse",
            additionalContext: `Formatters processed changed files: ${changed.join(", ")}. Read the final content before further edits.`,
          },
        }),
      );
  } else if (mode === "openapi") {
    const changed = run(
      "git",
      ["diff", "--name-only", "-z", "HEAD"],
      root,
    ).split("\0");
    changed.push(
      ...run(
        "git",
        ["ls-files", "--others", "--exclude-standard", "-z"],
        root,
      ).split("\0"),
    );
    // Include committed feature work when the working tree is already clean.
    const base = spawnSync("git", ["merge-base", "HEAD", "origin/main"], {
      cwd: root,
      encoding: "utf8",
    });
    if (base.status === 0)
      changed.push(
        ...run(
          "git",
          ["diff", "--name-only", "-z", base.stdout.trim(), "HEAD"],
          root,
        ).split("\0"),
      );
    if (
      !changed.some(
        (file) =>
          /^backend\/.*\.(go|json|ya?ml)$/.test(file) ||
          /^backend\/go\.(mod|sum)$/.test(file) ||
          file.startsWith(".agents/skills/openapi-contract/scripts/"),
      )
    ) {
      console.log("{}");
      process.exit(0);
    }
    run(
      "sh",
      [".agents/skills/openapi-contract/scripts/check-openapi.sh"],
      root,
      110000,
    );
    console.log("{}");
  } else {
    throw new Error(`Unknown hook mode: ${mode}`);
  }
} catch (error) {
  const reason = `Codex ${mode} hook: ${error.message.slice(0, 6000)}. Fix the failure and rerun the check; do not claim validation succeeded.`;
  // A second failure is surfaced without causing an endless Stop continuation loop.
  console.log(
    JSON.stringify(
      mode === "openapi" && !event.stop_hook_active
        ? { decision: "block", reason }
        : {
            systemMessage: reason,
            ...(mode === "format"
              ? {
                  hookSpecificOutput: {
                    hookEventName: "PostToolUse",
                    additionalContext: reason,
                  },
                }
              : {}),
          },
    ),
  );
}
