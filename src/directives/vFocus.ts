import { App } from 'vue'
const focus = {
  mounted: (el: HTMLElement) => el.focus(),
}

export function setup(app: App) {
  app.directive('focus', focus)
}
