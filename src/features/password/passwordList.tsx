import { usePagination } from '@/hooks/usePagination'
import { findPassWordList } from '@/model/password'
import { useCategory } from '@/storage/category'
import { defineComponent, onMounted, reactive, ref, watch } from 'vue'
import {
  InputSearch,
  Button,
  List,
  ListItem,
  ListItemMeta,
} from 'ant-design-vue'
import AddIcon from '@/icons/add.vue'
import AddPasswordModal from './AddPasswordModal'
import RefreshIcon from '@/icons/refresh.vue'
import { GetPromiseReturns } from '@/types/utils'
import { useDescription } from '@/storage/description'
export default defineComponent({
  setup() {
    const { activeCategory } = useCategory()
    const pagination = usePagination()
    const handlePaginationChange = (page: number, size: number) => {
      pagination.page = page
      pagination.size = size
      getData()
    }
    const visible = ref<boolean>(false)
    const total = ref(0)
    const state = reactive({
      data: [] as GetPromiseReturns<typeof findPassWordList>[1],
    })

    const { setActiveDescription } = useDescription()

    const showModal = () => {
      visible.value = true
    }
    const hideModal = () => {
      visible.value = false
    }

    watch(
      () => activeCategory.value,
      () => {
        getData()
      }
    )
    async function getData() {
      const [t, result] = await findPassWordList({ ...pagination, cid: activeCategory.value })
      total.value = t
      state.data = result
    }

    onMounted(() => {
      getData()
    })
    const handleSelectActivePasswordDescription = (id: number) => {
      setActiveDescription(String(id))
    }
    return () => (
      <>
        <div>
          <InputSearch style="width: 200px" />
          <Button class="m-l-2" type="dashed" onClick={showModal}>
            {{ icon: () => <AddIcon /> }}
          </Button>
          <Button class="m-l-2" type="dashed" onClick={getData}>
            {{ icon: () => <RefreshIcon /> }}
          </Button>
        </div>
        <div class={'flex-1 overflow-auto p-b-4'}>
          <List
            item-layout="horizontal"
            dataSource={state.data}
            pagination={{
              ...pagination,
              size: String(pagination.size),
              total: total.value,
              onChange: handlePaginationChange,
            }}
          >
            {state.data.map((item) => {
              return (
                <ListItem
                  key={item.id}
                  class="cursor-pointer"
                  onClick={() => handleSelectActivePasswordDescription(item.id)}
                >
                  <ListItemMeta description={item.username}>
                    {{
                      title: () => <span>{item.description}</span>,
                    }}
                  </ListItemMeta>
                </ListItem>
              )
            })}
          </List>
        </div>
        <AddPasswordModal
          visible={visible.value}
          onCancel={hideModal}
          onRefresh={getData}
        />
      </>
    )
  },
})
