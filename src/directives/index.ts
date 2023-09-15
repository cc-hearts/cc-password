import { App } from "vue";


export function registerDirective(app: App) {
  Object.entries(import.meta.glob('./*.ts', { eager: true })).forEach(([path, Module]) => {
    console.log(path, Module);
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    Module.setup?.(app)
  })
}