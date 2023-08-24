import { execSync } from 'node:child_process'
import { join } from '../utils/path.js'
function copyDir(dirs: string, targetDirs: string) {
  execSync(`cp -r ${dirs} ${targetDirs}`)
}

export function setupPrismaClient() {
  // copy client dirs
  copyDir(join('prisma'), join('dist/prisma'))
  copyDir(join('scripts/prisma-client-js'), join('dist/prisma-client-js'))
}
