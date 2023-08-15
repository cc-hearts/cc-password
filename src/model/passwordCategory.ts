import { AddPasswordCategory } from '@/features/passwordCategory/types'
import { getModelInstance } from './init'
import { useUid } from '@/hooks'

function getModel() {
  return getModelInstance('passwordCategory')!
}

async function addPasswordCategory(data: AddPasswordCategory) {
  const uid = useUid()
  return getModel().create({
    data: {
      ...data,
      uid
    },
  })
}

async function searchPasswordCategory() {
  const uid = useUid()
  return await getModel().findMany({
    where: { uid }
  })
}

async function removePasswordCategory(id: number) {
  return getModel().delete({
    where: {
      id,
    },
  })
}

export { addPasswordCategory, searchPasswordCategory, removePasswordCategory }
