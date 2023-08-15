/**
 * replace npx prisma generate, use pnpm schema
 */
const { join } = require('node:path')
const { readFileSync, writeFileSync } = require('node:fs')
const { execSync } = require('child_process')
const { parse } = require('yaml')

function getOutput(recourse: string) {
  const reg = new RegExp(`output\\s*=\\s*["'](.*)["']`, 'gm')
  const matchResult = reg.exec(recourse)
  if (matchResult?.[1]) return matchResult[1]
}
function replaceEnv(recourse: string, replaceString: string) {
  const reg = new RegExp(`env\\(["'](.*)["']\\)`, 'gm')
  return recourse.replace(reg, replaceString)
}
function bootStrap() {
  execSync('npx prisma generate', { stdio: 'inherit' })
  const prismaPath = './prisma/schema.prisma'
  const data = readFileSync(prismaPath, 'utf-8')
  const outputPath = getOutput(data)
  // output prisma path
  const prismaClientJsPath = join(prismaPath, '..', outputPath)
  const pkgSchemaPath = `./${prismaClientJsPath}/schema.prisma`
  const prismaSchema = readFileSync(pkgSchemaPath, 'utf-8')
  const env = readFileSync(join(process.cwd(), 'prisma.yaml'), 'utf-8')
  if (env) {
    const { DATABASE_URL } = parse(env)
    const newPrismaSchema = replaceEnv(prismaSchema, `"${DATABASE_URL}"`)
    writeFileSync(pkgSchemaPath, newPrismaSchema)
  }
}

bootStrap()
