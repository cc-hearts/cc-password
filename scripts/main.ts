import { app, BrowserWindow, type BrowserWindowConstructorOptions } from 'electron';
import { setup } from './utils/preset';
import { registerScheme } from './customScheme.js'
setup();
let mainBrowserWindow: BrowserWindow | null = null;
app.whenReady().then(() => {
  const webPreferences: BrowserWindowConstructorOptions['webPreferences'] = {
    nodeIntegration: true,
    webSecurity: false,
    allowRunningInsecureContent: true,
    contextIsolation: false,
    webviewTag: true,
    spellcheck: false,
    disableHtmlFullscreenWindowResize: true,
  }
  mainBrowserWindow = new BrowserWindow({ webPreferences })
  mainBrowserWindow.webContents.openDevTools({ mode: 'undocked' });
  console.log('process.argv', process.argv);
  if (process.argv[2]) {
    mainBrowserWindow.loadURL(process.argv[2]);
  } else {
    registerScheme()
    mainBrowserWindow.loadURL('app://index.html')
  }
})