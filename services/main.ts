import {
  app,
  BrowserWindow,
  type BrowserWindowConstructorOptions,
} from 'electron'
import { setup } from '../scripts/utils/preset'
import { registerScheme } from './custom-scheme.js'
import { registerEvent } from './events.js'
import { registerTray } from './tray'
import { setMainBrowserWindow, getMainBrowserWindow } from './context'
setup()

app.whenReady().then(() => {
  registerTray()
  const webPreferences: BrowserWindowConstructorOptions['webPreferences'] = {
    nodeIntegration: true,
    webSecurity: false,
    allowRunningInsecureContent: true,
    contextIsolation: false,
    webviewTag: true,
    spellcheck: false,
    disableHtmlFullscreenWindowResize: true,
    preload: `${__dirname}/preload.js`,
  }
  const mainBrowserWindow = new BrowserWindow({
    webPreferences,
    show: false,
    frame: false,
  })
  setMainBrowserWindow(mainBrowserWindow)
  const IS_DEV = !!process.argv[2]
  if (IS_DEV) {
    mainBrowserWindow.webContents.openDevTools({ mode: 'undocked' })
  }
  registerEvent()
  if (IS_DEV) {
    mainBrowserWindow.loadURL(process.argv[2])
  } else {
    registerScheme()
    mainBrowserWindow.loadURL('app://index.html')
  }
})

app.on('activate', () => {
  getMainBrowserWindow()?.show()
})
