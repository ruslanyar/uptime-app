import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: process.env.E2E_DIST_DIR ?? ".next",
  allowedDevOrigins: ["frontend.auth-client.test"],
  reactCompiler: true,

};

export default nextConfig;
