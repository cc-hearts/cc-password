const PrismaClient = require('./prisma-client-js').PrismaClient

const prisma = new PrismaClient()

window.__API__ = {
  prisma,
}
