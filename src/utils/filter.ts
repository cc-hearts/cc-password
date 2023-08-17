export function filterFalsy(target: Record<string, unknown>) {
  return Object.keys(target).reduce((acc, key) => {
    if (target[key]) {
      Reflect.set(acc, key, target[key])
    }
    return acc
  }, {})
}
