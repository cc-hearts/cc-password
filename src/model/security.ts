import { AddSecurity } from '@/features/security/types'
import { getModelInstance } from './init'

async function findSecurity(uid: number) {
  const securityModel = getModelInstance('security')!
  return await securityModel.findFirst({ where: { uid } })
}

async function addSecurity(securityModel: AddSecurity) {
  const model = getModelInstance('security')!
  return await model.create({
    data: securityModel,
  })
}
export { findSecurity, addSecurity }
