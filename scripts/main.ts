import {
  app,
  BrowserWindow,
  type BrowserWindowConstructorOptions,
} from 'electron'
import { setup } from './utils/preset'
import { registerScheme } from './customScheme.js'
setup()
let mainBrowserWindow: BrowserWindow | null = null
app.whenReady().then(() => {
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
  mainBrowserWindow = new BrowserWindow({ webPreferences })
  const IS_DEV = !!process.argv[2]
  if (IS_DEV) {
    mainBrowserWindow.webContents.openDevTools({ mode: 'undocked' })
  }
  if (IS_DEV) {
    mainBrowserWindow.loadURL(process.argv[2])
  } else {
    registerScheme()
    mainBrowserWindow.loadURL('app://index.html')
  }
})
