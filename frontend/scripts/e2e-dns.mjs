// Next's image optimizer needs fixture DNS too; Chromium's resolver rules do not affect Node.
import dns from 'node:dns';
import { syncBuiltinESMExports } from 'node:module';

const domains = new Set(['frontend.auth-client.test', 'api.auth-service.test']);
const lookup = dns.lookup.bind(dns);
const promiseLookup = dns.promises.lookup.bind(dns.promises);
dns.lookup = (hostname, ...args) =>
  lookup(domains.has(hostname) ? '127.0.0.1' : hostname, ...args);
dns.promises.lookup = (hostname, ...args) =>
  promiseLookup(domains.has(hostname) ? '127.0.0.1' : hostname, ...args);

// Keep named ESM imports of lookup from bypassing the patched resolver.
syncBuiltinESMExports();
