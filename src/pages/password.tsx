import RemoveIcon from '@/icons/remove.vue'
import { useI18n } from 'vue-i18n'
import { useNamespace } from '@/hooks'
import { successMsg } from '@/utils/message'
import { useCategory } from '@/storage/category'
import { IEvent } from '@/types/common'
import { GetPromiseReturns } from '@/types/utils'
import { Input, Modal } from 'ant-design-vue'
import { isNull } from '@cc-heart/utils'
import { ref, defineComponent, onMounted, onUnmounted } from 'vue'
import {
  addPasswordCategory,
  removePasswordCategory,
  searchPasswordCategory,
} from '@/model/passwordCategory'
import PasswordList from '@/features/password/passwordList'
import PasswordDescription from '@/features/password/passwordDescription'
import '@/assets/scss/pages/password.scss'

export default defineComponent({
  setup() {
    const { t } = useI18n()
    const ns = useNamespace('category')
    const isAddStatus = ref(false)
    const addCategory = ref('')
    const ALL_CATEGORY = { id: 0, category: t('pages-password.allCategory') }
    const {
      category: categoryList,
      setCategory,
      activeCategory,
      setActiveCategory,
    } = useCategory<GetPromiseReturns<typeof searchPasswordCategory>>()
    const toggleAddStatus = () => {
      isAddStatus.value = !isAddStatus.value
    }

    const handleEnter = async () => {
      if (!addCategory.value.trim()) return
      await addPasswordCategory({ category: addCategory.value })
      addCategory.value = ''
      toggleAddStatus()
      getData()
    }

    const handleKeyDowns = (event: KeyboardEvent) => {
      if (event.code === 'Escape' && isAddStatus.value) {
        toggleAddStatus()
      }
    }

    async function getData() {
      const data = await searchPasswordCategory()
      setCategory([ALL_CATEGORY, ...data])
      if (isNull(activeCategory.value)) {
        setActiveCategory(ALL_CATEGORY.id)
      }
    }

    async function stopPropagationEvent(event: MouseEvent) {
      event.stopPropagation()
    }

    function listenCancelClick() {
      if (isAddStatus.value) toggleAddStatus()
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
    const handleActiveCategory = (id: number) => {
      setActiveCategory(id)
    }

    onMounted(() => {
      getData()
      window.addEventListener('click', listenCancelClick)
    })

    onUnmounted(() => {
      window.removeEventListener('click', listenCancelClick)
    })
    return () => (
      <div class="flex h-full w-full">
        <div class="w-1/4 p2 will-change-transform	">
          <ul class="flex items-center justify-center flex-col p-x-6">
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
                    {item.category}
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
                    onChange={(e: IEvent<HTMLInputElement>) =>
                      (addCategory.value = e.target?.value)
                    }
                    allowClear
                    onPressEnter={handleEnter}
                  ></Input>
                </div>
              )}
            </div>
          </ul>
        </div>
        <div class="w-1/3 min-w-xs p-x-2 flex flex-col border-x-1 border-ins border-x-solid">
          <PasswordList />
        </div>
        <div class="flex-1">
          <PasswordDescription />
        </div>
      </div>
    )
  },
})
