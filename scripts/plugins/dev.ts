import { ViteDevServer } from 'vite'
import { esbuildConfig } from './config'
export const devPlugin = () => {
  return {
    name: 'dev-plugin',
    async configureServer(server: ViteDevServer) {
      const esbuild = await import('esbuild')
      esbuild.buildSync(esbuildConfig)
      server.httpServer?.once('listening', async () => {
        const { spawn } = await import('child_process')
        const addressInfo = server.httpServer?.address()!
        console.log('address info', addressInfo)
        if (typeof addressInfo !== 'string') {
          const httpAddress = `http://${addressInfo?.address}:${addressInfo.port}`
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
