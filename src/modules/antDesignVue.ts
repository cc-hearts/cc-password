import { App } from 'vue'
import Antd from 'ant-design-vue'
import 'ant-design-vue/dist/reset.css'

export const setup = ({ app }: { app: App }) => {
  app.use(Antd)
}
