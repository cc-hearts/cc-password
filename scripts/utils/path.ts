import { join as _join } from 'node:path'
export function join(...paths: string[]) {
  return _join(process.cwd(), ...paths)
}
