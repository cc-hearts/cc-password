import { type Platform } from 'esbuild'
export const esbuildConfig = {
  entryPoints: ['./scripts/main.ts'],
  bundle: true,
  platform: 'node' as Platform,
  outfile: './dist/main.js',
  external: ['electron'],
}
