import { usePagination } from '@/hooks/usePagination'
import { findPassWordList } from '@/model/password'
import { useCategory } from '@/storage/category'
import { computed, defineComponent, onMounted, reactive, ref, watch } from 'vue'
import {
  InputSearch,
  Button,
  List,
  ListItem,
  ListItemMeta,
  Spin,
} from 'ant-design-vue'
import AddIcon from '@/icons/add.vue'
import AddPasswordModal from './AddPasswordModal'
import RefreshIcon from '@/icons/refresh.vue'
import { GetPromiseReturns } from '@/types/utils'
import { useDescription } from '@/storage/description'
import { successTips } from '@/utils/message'
import { useI18n } from 'vue-i18n'
import { ChangeEvent } from 'ant-design-vue/es/_util/EventInterface'

export default defineComponent({
  setup() {
    const { activeCategory } = useCategory()
    const pagination = usePagination()
    const { t } = useI18n()
    const handlePaginationChange = (page: number, size: number) => {
      pagination.page = page
      pagination.size = size
      getData()
    }
    const visible = ref<boolean>(false)
    const total = ref(0)
    const searchData = ref('')
    const state = reactive({
      loading: false,
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
        handlePaginationChange(1, pagination.size)
      }
    )
    async function getData() {
      state.loading = true
      try {
        const [t, result] = await findPassWordList(
          {
            ...pagination,
            cid: activeCategory.value,
          },
          searchData.value
        )
        total.value = t
        state.data = result
      } catch (e) {
        console.log('[passwordList] getData error', e)
      } finally {
        state.loading = false
      }
    }

    onMounted(() => {
      getData()
    })
    const handleSelectActivePasswordDescription = (id: number) => {
      setActiveDescription(String(id))
    }

    const refreshData = async () => {
      await getData()
      successTips(t('passwordList.refreshSuccessTips'))
    }

    const paginationProps = computed(() => {
      if (total.value > pagination.size) {
        return {
          ...pagination,
          size: String(pagination.size) as 'small' | 'default',
          total: total.value,
          onChange: handlePaginationChange,
        }
      }
      return false
    })
    return () => (
      <>
        <div class="m-t-4">
          <InputSearch
            style="width: 200px"
            value={searchData.value}
            onChange={(e: ChangeEvent) => (searchData.value = e.target.value!)}
            onSearch={getData}
          />
          <Button class="m-l-2" type="dashed" onClick={showModal}>
            {{ icon: () => <AddIcon /> }}
          </Button>
          <Button class="m-l-2" type="dashed" onClick={refreshData}>
            {{ icon: () => <RefreshIcon /> }}
          </Button>
        </div>
        <div class={'flex-1 overflow-auto p-b-4'}>
          <Spin spinning={state.loading}>
            <List
              item-layout="horizontal"
              dataSource={state.data}
              pagination={paginationProps.value}
            >
              {state.data.map((item) => {
                return (
                  <ListItem
                    key={item.id}
                    class="cursor-pointer"
                    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                    // @ts-ignore
                    onClick={() => {
                      handleSelectActivePasswordDescription(item.id)
                    }}
                  >
                    <ListItemMeta description={item.username}>
                      {{
                        title: () => <span>{item.title}</span>,
                      }}
                    </ListItemMeta>
                  </ListItem>
                )
              })}
            </List>
          </Spin>
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
