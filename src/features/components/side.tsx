import { useCssNamespace } from '@/hooks'
import RemoveIcon from '@/icons/remove.vue'
import { searchPasswordCategory } from '@/model/password-category'
import { useCategory } from '@/storage'
import { GetPromiseReturns } from '@/types/utils'
import { successMsg } from '@/utils/message'
import { isNull } from '@cc-heart/utils'
import { Input, Modal } from 'ant-design-vue'
import { ChangeEvent } from 'ant-design-vue/es/_util/EventInterface'
import { defineComponent, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import TopBar from '@/components/top-bar/index.vue'
import {
  addPasswordCategory,
  removePasswordCategory,
} from '@/model/password-category'

export default defineComponent({
  name: 'Side',
  setup() {
    const { t } = useI18n()
    const ns = useCssNamespace('category')
    const isAddStatus = ref(false)
    const addCategory = ref('')
    const ALL_CATEGORY = { id: 0, category: '' }
    const {
      category: categoryList,
      setCategory,
      activeCategory,
      setActiveCategory,
    } = useCategory<GetPromiseReturns<typeof searchPasswordCategory>>()

    const handleActiveCategory = (id: number) => {
      setActiveCategory(id)
    }

    async function getData() {
      const data = await searchPasswordCategory()
      setCategory([ALL_CATEGORY, ...data])
      if (isNull(activeCategory.value)) {
        setActiveCategory(ALL_CATEGORY.id)
      }
    }

    async function removeCategory(id: number) {
      await removePasswordCategory(id)
      successMsg(t('pages-password.removeCategorySuccessMessage'))
      getData()
    }

    const handleRemoveCategory = (id: number) => {
      Modal.confirm({
        title: () => t('pages-password.removeCategoryConfirmTitle'),
        onOk: () => removeCategory(id),
      })
    }

    const toggleAddStatus = () => {
      isAddStatus.value = !isAddStatus.value
    }

    const handleKeyDowns = (event: KeyboardEvent) => {
      if (event.code === 'Escape' && isAddStatus.value) {
        toggleAddStatus()
      }
    }

    const handleEnter = async () => {
      if (!addCategory.value.trim()) return
      await addPasswordCategory({ category: addCategory.value })
      addCategory.value = ''
      toggleAddStatus()
      getData()
    }

    async function stopPropagationEvent(event: MouseEvent) {
      event.stopPropagation()
    }

    function listenCancelClick() {
      if (isAddStatus.value) toggleAddStatus()
    }

    onMounted(() => {
      getData()
      window.addEventListener('click', listenCancelClick)
    })

    onUnmounted(() => {
      window.removeEventListener('click', listenCancelClick)
    })

    return () => (
      <div class="w-1/4 will-change-transform h-full flex flex-col">
        <div
          class={
            'h-49px border-b-1 border-b-solid border-ins flex items-center p-l-4 app-drag'
          }
        >
          <TopBar />
        </div>
        <div class="flex-1 border-r-1 border-ins border-x-solid flex items-center flex-col p2 p-x-6">
          <ul>
            {categoryList.value.map((item) => {
              return (
                <li
                  class={
                    'm-y-2 w-full text-center cursor-pointer relative ' +
                    ns.e('item')
                  }
                  key={item.id}
                  onClick={() => handleActiveCategory(item.id)}
                >
                  <div
                    class={
                      String(activeCategory.value) === String(item.id)
                        ? ns.em('item', 'active')
                        : ''
                    }
                  >
                    {item.id === 0
                      ? t('pages-password.allCategory')
                      : item.category}
                    {item.id !== 0 && (
                      <span
                        class="absolute right-0"
                        onClick={() => handleRemoveCategory(item.id)}
                      >
                        <RemoveIcon />
                      </span>
                    )}
                  </div>
                </li>
              )
            })}
            <div class="m-y-2" onClick={stopPropagationEvent}>
              {!isAddStatus.value ? (
                <li class="cursor-pointer" onClick={toggleAddStatus}>
                  <span>{t('pages-password.newCategory')}</span>
                  <span class="m-l-1">+</span>
                </li>
              ) : (
                <div onKeydown={handleKeyDowns}>
                  <Input
                    value={addCategory.value}
                    onChange={(e: ChangeEvent) =>
                      (addCategory.value = e.target?.value!)
                    }
                    allowClear
                    onPressEnter={handleEnter}
                  ></Input>
                </div>
              )}
            </div>
          </ul>
        </div>
      </div>
    )
  },
})
