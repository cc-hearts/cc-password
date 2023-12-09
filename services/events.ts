// showWindow event
import { BrowserWindow, dialog, ipcMain } from 'electron'

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

export function handleSelectFilePathEvent() {
  ipcMain.handle('open-search-dir-path', (event) => {
    dialog
      .showOpenDialog({
        properties: ['openDirectory'],
        title: 'Select Directory',
        buttonLabel: 'save',
      })
      .then((res) => {
        console.log(res)
        const { canceled } = res
        if (!canceled) {
          const [path] = res.filePaths
          event.sender.send('selected-dir-path', path)
        }
      })
  })
}

export function registerEvent() {
  ;[
    handleSelectFilePathEvent,
    handleShowWindowEvent,
    handleCloseWindowEvent,
    handleChangeWindowSizeEvent,
  ].forEach((fn) => fn())
}
