import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  distDir: process.env.E2E_DIST_DIR ?? '.next',
  allowedDevOrigins: ['frontend.auth-client.test'],
  reactCompiler: true,
  // The shared launcher uses one hostname for CORS, SameSite cookies and tab locks.
  redirects() {
    if (process.env.LOCAL_DEV_LAUNCH !== '1') return [];
    return [
      {
        source: '/:path*',
        has: [{ type: 'host', value: '127\\.0\\.0\\.1' }],
        destination: 'http://localhost:3000/:path*',
        permanent: false,
      },
    ];
  },
};

export default nextConfig;
