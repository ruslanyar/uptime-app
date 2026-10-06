import type { NextConfig } from 'next';
import { apiOrigin } from './src/lib/auth/api';

const avatarOrigin = process.env.NEXT_PUBLIC_API_URL
  ? new URL(apiOrigin(process.env.NEXT_PUBLIC_API_URL))
  : undefined;

const nextConfig: NextConfig = {
  distDir: process.env.E2E_DIST_DIR ?? '.next',
  allowedDevOrigins: ['frontend.auth-client.test'],
  reactCompiler: true,
  images: {
    remotePatterns: avatarOrigin
      ? [new URL('/api/v1/avatars/**', avatarOrigin)]
      : [],
    // Development APIs run on loopback; remotePatterns still restricts the source.
    dangerouslyAllowLocalIP:
      process.env.NODE_ENV === 'development' ||
      ['localhost', '127.0.0.1', '[::1]'].includes(
        avatarOrigin?.hostname ?? '',
      ),
    maximumRedirects: 0,
    maximumResponseBody: 500 * 1024,
  },
  // The shared launcher uses one hostname for CORS, SameSite cookies and tab locks.
  redirects() {
    if (process.env.LOCAL_DEV_LAUNCH !== '1') return [];
    return [
      {
        source: '/:path*',
        has: [{ type: 'host', value: '127\\.0\\.0\\.1' }],
        destination: `http://localhost:${process.env.PORT}/:path*`,
        permanent: false,
      },
    ];
  },
};

export default nextConfig;
