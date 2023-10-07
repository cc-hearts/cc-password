import { ViteDevServer } from 'vite'
import { esbuildConfig } from './config'
import { copy } from './preloadCopy'
export const devPlugin = () => {
  return {
    name: 'dev-plugin',
    async configureServer(server: ViteDevServer) {
      const esbuild = await import('esbuild')
      esbuild.buildSync(esbuildConfig)
      server.httpServer?.once('listening', async () => {
        const { spawn } = await import('child_process')
        await copy()
        const addressInfo = server.httpServer?.address()!
        console.log('address info', addressInfo)
        if (typeof addressInfo !== 'string') {
          const address = addressInfo?.address.includes('::')
            ? 'localhost'
            : addressInfo?.address
          const httpAddress = `http://${address}:${addressInfo.port}`
          const electronProcess = spawn(
            (await import('electron')).default.toString(),
            ['./dist/main.js', httpAddress],
            {
              cwd: process.cwd(),
              stdio: 'inherit',
            }
          )
          electronProcess.on('close', () => {
            server.close()
            process.exit()
          })
        }
      })
    },
  }
}
