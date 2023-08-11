export const getReplacer = () => {
  const externals = ['os', 'fs', 'path', 'events', 'http', 'buffer', 'url', 'util', 'child_process', 'sandbox']
  const res = {}
  externals.reduce((acc, external) => {
    Reflect.set(acc, external, {
      find: new RegExp(`^${external}$`),
      code: `const ${external} = require('${external}'); export { ${external} as default }`
    })
    return acc
  }, res)
  return res
}