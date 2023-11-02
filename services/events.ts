// showWindow event
import { BrowserWindow, ipcMain } from 'electron'

// get BrowserWindow Instance
function getCurrentInstance(e: Electron.IpcMainInvokeEvent) {
  return BrowserWindow.fromWebContents(e.sender)
}
export function handleShowWindowEvent() {
  ipcMain.handle('show-window', (e) => {
    const ctx = getCurrentInstance(e)
    ctx && ctx.show()
  })
}

export function handleCloseWindowEvent() {
  ipcMain.handle('hidden-window', (e) => {
    const ctx = getCurrentInstance(e)
    ctx && ctx.hide()
  })
}

export function handleChangeWindowSizeEvent() {
  let isMaximized = false
  ipcMain.handle('change-window-size', (e) => {
    const ctx = getCurrentInstance(e)
    if (isMaximized) {
      ctx && ctx.unmaximize()
    } else {
      ctx && ctx.maximize()
    }
    isMaximized = !isMaximized
  })
}

export function registerEvent() {
  handleShowWindowEvent()
  handleCloseWindowEvent()
  handleChangeWindowSizeEvent()
}
