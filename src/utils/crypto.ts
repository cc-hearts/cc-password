import crypto from 'crypto'
import { useProfile } from '@/storage/user'
const { createHash, createCipheriv, createDecipheriv, randomBytes } = crypto

export function encodeMd5(str: string) {
  const md5 = createHash('md5')
  const encodeStr = md5.update(str)
  return encodeStr.digest('hex')
}

// aes 加密
export function encodeAes(plainText: string) {
  const { security } = useProfile()
  if (!security.value) return null
  // 创建加密器
  const cipher = createCipheriv(
    'aes-256-cbc',
    strParseBuffer(security.value.key),
    strParseBuffer(security.value.iv)
  )
  // 加密
  let encrypted = cipher.update(plainText, 'utf8', 'hex')
  encrypted += cipher.final('hex')
  return encrypted
}
export function decodeAes(encrypted: string) {
  const { security } = useProfile()
  if (!security.value) return null
  const decipher = createDecipheriv(
    'aes-256-cbc',
    strParseBuffer(security.value.key),
    strParseBuffer(security.value.iv)
  )
  let decrypted = decipher.update(encrypted, 'hex', 'utf8')
  decrypted += decipher.final('utf8')
  return decrypted
}

export function strParseBuffer(str: string) {
  return Buffer.from(str, 'hex')
}

function generatorRandomBytes(count: number, isToString = false) {
  const key = randomBytes(count)
  if (isToString) return key.toString('hex')
  return key
}

function generatorKey() {
  return generatorRandomBytes(32, true)
}

function generatorIv() {
  return generatorRandomBytes(16, true)
}

export function generatorAesSign() {
  const key = generatorKey()
  const iv = generatorIv()
  return { key, iv }
}
