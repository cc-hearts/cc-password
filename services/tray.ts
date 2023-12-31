import { Menu, Tray, clipboard, nativeImage } from 'electron'
import { resolve } from 'path'
import { getMainBrowserWindow } from './context'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { generatorPassword } from '../federal/utils/crypto'

function genPasswordCopy() {
  const pwd = generatorPassword()
  clipboard.writeText(pwd)
}

function exitProgress() {
  // 关闭electron
  console.log(getMainBrowserWindow());
  getMainBrowserWindow()?.close()
  process.exit(0)
}

export function registerTray() {
  const icon = nativeImage.createFromPath(resolve(__dirname, './icon.png'))
  const tray = new Tray(icon)
  const contextMenu = Menu.buildFromTemplate([
    { label: '密码生成', type: 'normal', click: genPasswordCopy },
    { label: '退出应用', type: 'normal', click: exitProgress }
  ])
  tray.setContextMenu(contextMenu)
}
