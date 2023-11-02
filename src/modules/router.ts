import { isDev } from '@/configs'
import { App } from 'vue'
import { RouteRecordRaw, createRouter, createWebHashHistory } from 'vue-router'
import routes from '~pages'

const whiteRouterPathList = ['/login']

const indexRoutes = {
  path: '/',
  name: 'home',
  component: () => import('@/pages/index.vue'),
  children: [] as RouteRecordRaw[],
}
let _routes = routes.filter((item) => item.path !== '/')
const whiteRouterList = _routes.filter((item) =>
  whiteRouterPathList.includes(item.path)
)
const childrenRouterList = _routes.filter(
  (item) => !whiteRouterPathList.includes(item.path)
)
indexRoutes.children = childrenRouterList.map((item) => {
  item.path = item.path.replace('/', '')
  return item
})

_routes = [...whiteRouterList, indexRoutes]
export const router = createRouter({
  history: createWebHashHistory(),
  routes: _routes,
})

export const setup = ({ app }: { app: App }) => {
  if (isDev) {
    console.log(_routes)
  }
  app.use(router)
}
