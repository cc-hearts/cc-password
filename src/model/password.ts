import { InsertPassword } from '@/types'
import { getModelInstance } from './init'
import type { Pagination } from '@/types/pagination'
import { useTransformPagination } from '@/hooks/usePagination'

async function getModel() {
  return getModelInstance('password')!
}
async function findPassWordList<T extends Pagination>(params: T) {
  const { page, size, ...where } = params
  const { take, skip } = useTransformPagination({ page, size })
  const model = await getModel()
  return await Promise.all([
    model.count(),
    model.findMany({ where, take, skip }),
  ])
}

async function addPassWord<T extends InsertPassword>(data: T) {
  const model = await getModel()
  return await model.create({ data })
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
      description: true,
    },
  })
}

async function searchPassword(id: number) {
  const model = await getModel()
  return await model.findUnique({ where: { id } })
}

export { findPassWordList, addPassWord, findPasswordDetail, searchPassword }
