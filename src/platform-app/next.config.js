/** @type {import('next').NextConfig} */
const nextConfig = {
  // REQUIRED by docker/react.dockerfile — generates .next/standalone for the Docker image
  output: 'standalone',
  reactStrictMode: true,
};

module.exports = nextConfig;
