// showWindow event
import { BrowserWindow, ipcMain } from 'electron'

// get BrowserWindow Instance
function getCurrentInstance(e: Electron.IpcMainInvokeEvent) {
  return BrowserWindow.fromWebContents(e.sender)
}
export function handleShowWindowEvent() {
  ipcMain.handle('showWindow', (e) => {
    const ctx = getCurrentInstance(e)
    ctx && ctx.show()
  })
}
