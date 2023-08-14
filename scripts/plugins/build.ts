import { esbuildConfig } from './config.js'
import { join as _join } from 'node:path'
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  createReadStream,
  createWriteStream,
} from 'node:fs'
import { execSync } from 'node:child_process'
function join(...paths: string[]) {
  return _join(process.cwd(), ...paths)
}
function build() {
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

  function copy() {
    return new Promise<void>((resolve) => {
      const readStream = createReadStream(join('scripts/preload.js'))
      const writeStream = createWriteStream(join('dist', 'preload.js'))
      readStream.pipe(writeStream)
      writeStream.on('finish', () => {
        resolve()
      })
    })
  }

  function copyDir(dirs: string, targetDirs: string) {
    execSync(`cp -r ${dirs} ${targetDirs}`)
  }

  function prepareNodeModules() {
    // node 复制文件夹
    copyDir(join('prisma'), join('dist/prisma'))
    copyDir(join('scripts/prisma-client-js'), join('dist/prisma-client-js'))
  }

  // 构建依赖
  function deployCode() {
    const options = {
      config: {
        directories: {
          output: join('release'),
          app: join('dist'),
        },
        files: ['**'],
        extends: null,
        productName: 'electron-password',
        appId: 'cc-hearts',
        asar: true,
        nsis: {
          oneClick: true,
          perMachine: true,
          allowToChangeInstallationDirectory: false,
          createDesktopShortcut: true,
          createStartMenuShortcut: true,
          shortcutName: 'cc',
        },
        publish: [{ provider: 'generic', url: 'http://localhost:5173/' }],
      },
      project: process.cwd(),
    }
    return require('electron-builder').build(options)
  }
  return {
    buildMain,
    prepareNodeModules,
    copy,
    preparePkg,
    deployCode,
  }
}

export const buildPlugin = () => {
  return {
    name: 'electron-build-plugin',
    async closeBundle() {
      // build end hook
      const {
        buildMain,
        copy,
        prepareNodeModules,
        preparePkg,
        deployCode,
      } = build()
      buildMain()
      prepareNodeModules()
      await copy()
      preparePkg()
      deployCode()
    },
  }
}
