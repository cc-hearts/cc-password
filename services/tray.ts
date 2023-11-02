import { nativeImage, Tray, Menu, clipboard } from 'electron'
import { resolve } from 'path'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { generatorPassword } from '../federal/utils/crypto'

function genPasswordCopy() {
  const pwd = generatorPassword()
  clipboard.writeText(pwd)
}

export function registerTray() {
  const icon = nativeImage.createFromPath(resolve(__dirname, './icon.png'))
  const tray = new Tray(icon)
  const contextMenu = Menu.buildFromTemplate([
    { label: '密码生成', type: 'normal', click: genPasswordCopy },
  ])
  tray.setContextMenu(contextMenu)
}
