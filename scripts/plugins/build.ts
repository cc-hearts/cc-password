import { esbuildConfig } from "./config.js"
import { join as _join } from 'node:path'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
function join(...paths: string[]) {
  return _join(process.cwd(), ...paths)
}
export function buildPlugin() {
  function buildMain() {
    require('esbuild').buildSync(esbuildConfig)
  }

  function preparePkg() {
    const pkgUrl = join('package.json')
    let pkg: string | Record<string, any> = readFileSync(pkgUrl, 'utf-8')
    pkg = JSON.parse(pkg)
    if (typeof pkg !== 'string') {
      const version = pkg.devDependencies.electron.replace('^', '')
      pkg.main = 'main.js'
      delete pkg.scripts
      pkg.devDependencies = { electron: version }
      const tarJsonPath = join('dist', 'package.json')
      writeFileSync(tarJsonPath, JSON.stringify(pkg, null, 2), 'utf-8')
      mkdirSync(join('dist', 'node_modules'))
    }
  }


  // 构建依赖
  function deployCode() {
    const options = {
      config: {
        directories: {
          output: join('release'),
          app: join('dist'),
        },
        files: ["**"],
        extends: null,
        productName: 'electron-app',
        appId: 'cc-hearts',
        asar: true,
        nsis: {
          oneClick: true,
          perMachine: true,
          allowToChangeInstallationDirectory: false,
          createDesktopShortcut: true,
          createStartMenuShortcut: true,
          shortcutName: "cc",
        },
        publish: [{ provider: "generic", url: "http://localhost:5500/" }],
      },
      project: process.cwd(),
    }
    return require('electron-builder').build(options)
  }
  return { buildMain, preparePkg, deployCode }
}

export default () => {
  return {
    name: 'electron-build-plugin',
    closeBundle() {
      // build end hook
      const { buildMain, preparePkg, deployCode } = buildPlugin()
      buildMain()
      preparePkg()
      deployCode()
    }
  }
}