import { PropType } from 'vue'
interface Options {
  label: string
  value: any
}
export interface Column {
  label: string
  type: string
  name: string
  width?: number
  fixed?: 'left' | 'right'
  slot?: { name: string; [props: string]: any }
  extra?: { [props: string]: any }
  options?: Options[]
}

export interface FormExpose {
  validate: () => Promise<boolean>
  getFieldsValue: <T extends Record<PropertyKey, any>>() => T
  setFieldValue: (key: PropertyKey, value: any) => void
  setFieldsValue: (target: Record<PropertyKey, any>) => void
  resetFields: () => void
}

export default {
  name: {
    type: String,
    default: 'basic',
  },
  labelCol: {
    type: Object,
    default: () => ({ span: 6 }),
  },
  columns: {
    type: Array as PropType<Column[]>,
    default: () => [],
  },
  defaultValue: {
    type: Object,
    default: () => ({}),
  },
  rules: {
    type: Object,
    default: () => ({}),
  },
}
