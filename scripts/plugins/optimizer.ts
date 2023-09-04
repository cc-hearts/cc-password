// https://github.com/vite-plugin/vite-plugin-optimizer/blob/main/README.md#how-to-work
export const getReplacer = () => {
  const externals = [
    'os',
    'fs',
    'path',
    'events',
    'http',
    'buffer',
    'url',
    'util',
    'child_process',
    'crypto',
    'process',
  ]
  const res = {}
  externals.reduce((acc, external) => {
    Reflect.set(acc, external, {
      find: new RegExp(`^${external}$`),
      code: `const ${external} = require('${external}'); export { ${external} as default }`,
    })
    return acc
  }, res)

  // electron api
  const electronApis = ['ipcRenderer', 'clipboard'].join(',')
  res['electron'] = {
    find: new RegExp(`^electron$`),
    code: `const {${electronApis}} = require('electron'); export {${electronApis}} `,
  }
  return res
}
