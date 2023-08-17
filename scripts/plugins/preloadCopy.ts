import { createReadStream, createWriteStream } from "node:fs"
import { join } from '../utils/path.js'
export function copy() {
  return new Promise<void>((resolve) => {
    const readStream = createReadStream(join('scripts/preload.js'))
    const writeStream = createWriteStream(join('dist', 'preload.js'))
    readStream.pipe(writeStream)
    writeStream.on('finish', () => {
      resolve()
    })
  })
}