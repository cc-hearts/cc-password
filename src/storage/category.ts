import { UnwrapNestedRefs, Ref, computed, reactive } from 'vue'

let state: null | UnwrapNestedRefs<any> = null

export function useCategory<T>() {
  if (!state) {
    state = reactive({
      data: [],
      activeCategory: 0,
    })
  }

  const category = computed(() => state.data) as Ref<T>

  const activeCategory = computed(() => state.activeCategory) as Ref<string>

  function setCategory<T>(data: T) {
    state.data = data
  }

  function setActiveCategory(category: number) {
    state.activeCategory = category
  }
  return { category, activeCategory, setCategory, setActiveCategory }
}
