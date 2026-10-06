import { parseEnv } from 'node:util';
import { readFileSync, existsSync, appendFileSync, chmodSync } from 'node:fs';
import { randomBytes } from 'node:crypto';

function settings(project) {
  const env = {};
  for (const file of ['.env', '.env.development', '.env.local', '.env.development.local']) {
    const path = new URL(`../${project}/${file}`, import.meta.url);
    if (existsSync(path)) Object.assign(env, parseEnv(readFileSync(path, 'utf8')));
  }
  return { ...env, ...process.env };
}
const backend = settings('backend');
const frontend = settings('frontend');
if (process.argv[2] === 'secret') {
  if (!backend.JWT_SECRET) {
    const oldSecret = new URL('../.dev/jwt-secret', import.meta.url);
    const secret = existsSync(oldSecret)
      ? readFileSync(oldSecret, 'utf8').trim()
      : randomBytes(32).toString('hex');
    const localFile = new URL('../backend/.env.development.local', import.meta.url);
    appendFileSync(localFile, `\nJWT_SECRET=${secret}\n`, { mode: 0o600 });
    chmodSync(localFile, 0o600);
  }
} else {
  console.log(`${backend.HTTP_ADDR.split(':').at(-1)} ${frontend.PORT}`);
}
