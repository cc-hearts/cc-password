const { randomBytes } = require('crypto')
export function generatorPassword(length = 16) {
  const chars =
    'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-='
  const _randomBytes = randomBytes(length)
  const passwordArray = new Array(length)

  for (let i = 0; i < length; i++) {
    passwordArray[i] = chars[_randomBytes[i] % chars.length]
  }

  return passwordArray.join('')
}
