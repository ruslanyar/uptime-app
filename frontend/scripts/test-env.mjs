import { createRequire } from 'node:module';
import { parseEnv } from 'node:util';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

// Test tools run outside Next.js. Keep test configuration isolated from .env.local.
process.env.NODE_ENV = 'test';
const { loadEnvConfig } = createRequire(`${process.cwd()}/package.json`)(
  '@next/env',
);
loadEnvConfig(process.cwd());

/** @returns {NodeJS.ProcessEnv} */
export function backendTestEnv() {
  const defaults = parseEnv(
    readFileSync(resolve('../backend/.env.test'), 'utf8'),
  );
  const local = resolve('../backend/.env.test.local');
  return {
    ...defaults,
    ...(existsSync(local) ? parseEnv(readFileSync(local, 'utf8')) : {}),
    ...process.env,
    APP_ENV: 'test',
  };
}
