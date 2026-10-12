import { rmSync } from 'node:fs';

rmSync(new URL('../.playwright-mcp/', import.meta.url), {
  recursive: true,
  force: true,
});
