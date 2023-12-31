import type { BrowserWindow } from "electron";

export let mainBrowserWindow: BrowserWindow | null = null


export function getMainBrowserWindow() {
  return mainBrowserWindow
}

export function setMainBrowserWindow(win: BrowserWindow) {
  mainBrowserWindow = win
}