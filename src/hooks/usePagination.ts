import { reactive } from 'vue'
import type { Pagination } from '@/types/pagination'

export function usePagination() {
  const pagination = reactive({
    page: 1,
    size: 10,
  })

  return pagination
}

export function useTransformPagination<T extends Pagination>(data: T) {
  const { page, size } = data
  const skip = (page - 1) * size
  return { skip, take: size }
}
