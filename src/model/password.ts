import { useUid } from '@/hooks'
import { useTransformPagination } from '@/hooks/use-pagination'
import { InsertPassword } from '@/types'
import type { Pagination } from '@/types/pagination'
import { filterFalsy } from '@/utils/filter'
import { getModelInstance } from './init'

async function getModel() {
  return getModelInstance('password')!
}
async function findPassWordList<T extends Pagination>(
  params: T,
  title?: string
) {
  const { page, size, ...target } = params
  const { take, skip } = useTransformPagination({ page, size })
  const model = await getModel()
  const uid = useUid()
  let where: { uid: number; title?: { contains: string } } = {
    ...filterFalsy(target),
    uid,
  }
  if (title) {
    where = {
      ...where,
      title: {
        contains: title,
      },
    }
  }

  return await Promise.all([
    model.count({ where }),
    model.findMany({ where, take, skip, orderBy: { createdAt: 'desc' } }),
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
      password: true,
      title: true,
      description: true,
      createdAt: true,
      updatedAt: true,
    },
  })
}

async function searchPassword(id: number) {
  const model = await getModel()
  return await model.findUnique({ where: { id } })
}

export async function changePasswordDescription<
  T extends Partial<Omit<InsertPassword, 'uid'>>
>(id: number, data: T) {
  const model = await getModel()
  return await model.update({
    where: { id },
    data,
  })
}

async function searchAllPassword() {
  const model = await getModel()
  const uid = useUid()
  const data = await model.findMany({ where: { uid } })
  return data
}

export {
  addPassWord,
  findPassWordList,
  findPasswordDetail,
  searchPassword,
  searchAllPassword,
}
