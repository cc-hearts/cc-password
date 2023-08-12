import { isNull } from '@cc-heart/utils'
import { type UnwrapNestedRefs, reactive, computed } from 'vue'

interface Description {
  active: string
}
let description: null | UnwrapNestedRefs<Description> = null

export function useDescription() {
  if (isNull(description)) {
    description = reactive({
      active: '',
    })
  }

  const activeDescription = computed(() => description!.active)

  const setActiveDescription = (active: string) => {
    description!.active = active
  }
  return { activeDescription, setActiveDescription }
}
