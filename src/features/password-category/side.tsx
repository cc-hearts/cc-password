import TopBar from '@/components/top-bar/index.vue'
import { useCssNamespace } from '@/hooks'
import { searchPasswordCategory } from '@/model/password-category'
import { useCategory } from '@/storage'
import { GetPromiseReturns } from '@/types/utils'
import {
  FolderOpenOutlined,
  FolderOutlined,
  PlusCircleFilled,
} from '@ant-design/icons-vue'
import { isNull } from '@cc-heart/utils'
import { defineComponent, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AddCategoryModal from './add-category-modal.vue'
import ContextMenu from './context-menu/context-menu.vue'
export default defineComponent({
  name: 'Side',
  setup() {
    const { t } = useI18n()
    const ns = useCssNamespace('category')
    const ALL_CATEGORY = { id: 0, category: '' }
    const addCategoryModalRef = ref()

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

    const handleRemoveAfterRefresh = () => {
      handleActiveCategory(ALL_CATEGORY.id)
      getData()
    }

    const toggleAddStatus = () => {
      addCategoryModalRef.value?.onOpen()
    }

    const isActiveCategoryItem = (id: number | string) =>
      String(activeCategory.value) === String(id)

    onMounted(() => {
      getData()
    })

    return () => (
      <>
        <div class="w-1/4 will-change-transform h-full flex flex-col shrink-0">
          <div class="h-49px border-b-1 border-b-solid border-ins flex items-center p-l-4 app-drag">
            <TopBar />
          </div>
          <div class="flex-1 border-r-1 border-ins border-x-solid items-center p2 p-b-12 p-x-6 relative">
            <ul class="w-full">
              {categoryList.value.map((item) => {
                return (
                  <li
                    class={
                      'm-y-2 w-full p-r-4 cursor-pointer relative ' +
                      ns.e('item')
                    }
                    key={item.id}
                    onClick={() => handleActiveCategory(item.id)}
                  >
                    <ContextMenu
                      id={item.id}
                      onRefreshCategoryList={handleRemoveAfterRefresh}
                    >
                      <div
                        class={
                          isActiveCategoryItem(item.id)
                            ? ns.em('item', 'active')
                            : ''
                        }
                      >
                        <span class="p-r-2">
                          {isActiveCategoryItem(item.id) ? (
                            <FolderOpenOutlined />
                          ) : (
                            <FolderOutlined />
                          )}
                        </span>
                        {item.id === 0
                          ? t('pages-password.allCategory')
                          : item.category}
                      </div>
                    </ContextMenu>
                  </li>
                )
              })}
            </ul>
            <div
              class="m-y-2 p-x-6 absolute bottom-2 left-0 w-full cursor-pointer"
              onClick={toggleAddStatus}
            >
              <PlusCircleFilled class="m-r-2" />
              <span>{t('pages-password.newCategory')}</span>
            </div>
          </div>
        </div>
        <AddCategoryModal
          ref={(e) => (addCategoryModalRef.value = e)}
          onRefresh={getData}
        />
      </>
    )
  },
})
