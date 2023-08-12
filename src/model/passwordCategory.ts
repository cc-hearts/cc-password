import { AddPasswordCategory } from '@/features/passwordCategory/types'
import { getModelInstance } from './init'

function getModel() {
  return getModelInstance('passwordCategory')!
}

async function addPasswordCategory(data: AddPasswordCategory) {
  return getModel().create({
    data,
  })
}

async function searchPasswordCategory() {
  return getModel().findMany()
}

async function removePasswordCategory(id: number) {
  return getModel().delete({
    where: {
      id,
    },
  })
}

export { addPasswordCategory, searchPasswordCategory, removePasswordCategory }
