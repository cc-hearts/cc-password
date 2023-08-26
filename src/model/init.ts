import { type PrismaClient as PrismaClientType } from '../../scripts/prisma-client-js'
let prisma: PrismaClientType | null = null

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
type filter<T> = T extends `$${infer r}` ? never : T extends symbol ? never : T
type modelKey = filter<keyof PrismaClientType>
function getPrismaInstance() {
  return (window as any).__API__.prisma as PrismaClientType
}

function destroyPrisma() {
  prisma?.$disconnect()
  prisma = null
}

function getModelInstance<T extends modelKey>(
  model: T
): PrismaClientType[T] | null {
  const prisma = getPrismaInstance()
  if (prisma === null) return null
  return prisma[model]
}
export { getPrismaInstance, destroyPrisma, getModelInstance }
