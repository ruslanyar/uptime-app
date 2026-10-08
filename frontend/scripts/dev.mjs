import { createRequire } from 'node:module';
import { spawn } from 'node:child_process';

// Local builds need development settings, while Next's build process must use production mode.
process.env.NODE_ENV = 'development';
const { loadEnvConfig } = createRequire(`${process.cwd()}/package.json`)(
  '@next/env',
);
loadEnvConfig(process.cwd(), true);
const build = process.argv[2] === 'build';
const child = spawn(
  process.execPath,
  [
    'node_modules/next/dist/bin/next',
    ...(build
      ? ['build', ...process.argv.slice(3)]
      : ['dev', '--port', process.env.PORT, ...process.argv.slice(2)]),
  ],
  {
    stdio: 'inherit',
    env: { ...process.env, NODE_ENV: build ? 'production' : 'development' },
  },
);
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => child.kill(signal));
}
child.on('error', (error) => {
  console.error(error.message);
  process.exitCode = 1;
});
child.on('exit', (code) => {
  process.exitCode = code ?? 1;
});
