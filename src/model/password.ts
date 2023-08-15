import { InsertPassword } from '@/types'
import { getModelInstance } from './init'
import type { Pagination } from '@/types/pagination'
import { useTransformPagination } from '@/hooks/usePagination'
import { filterFalsy } from '@/utils/filter'
import { useUid } from '@/hooks'

async function getModel() {
  return getModelInstance('password')!
}
async function findPassWordList<T extends Pagination>(params: T) {
  const { page, size, ...target } = params
  const { take, skip } = useTransformPagination({ page, size })
  const model = await getModel()
  const uid = useUid()
  const where = { ...filterFalsy(target), uid }
  return await Promise.all([
    model.count({ where }),
    model.findMany({ where, take, skip }),
  ])
}

async function addPassWord<T extends Omit<InsertPassword, 'uid'>>(data: T) {
  const uid = useUid()
  const model = await getModel()
  const newData: InsertPassword = { ...data, uid }
  return await model.create({ data: newData })
}

async function findPasswordDetail(id: number) {
  const model = await getModel()
  return await model.findUnique({
    where: { id },
    select: {
      id: true,
      uid: true,
      cid: true,
      url: true,
      username: true,
      password: false,
      description: true,
    },
  })
}

async function searchPassword(id: number) {
  const model = await getModel()
  return await model.findUnique({ where: { id } })
}

export { findPassWordList, addPassWord, findPasswordDetail, searchPassword }
