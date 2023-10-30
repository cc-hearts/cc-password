import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'
import { resolve } from 'path'
import vueJsx from '@vitejs/plugin-vue-jsx'
import VueI18n from '@intlify/unplugin-vue-i18n/vite'
import Pages from 'vite-plugin-pages'
import Markdown from 'vite-plugin-vue-markdown'
import LinkAttributes from 'markdown-it-link-attributes'
import Shiki from 'markdown-it-shiki'
import optimizer from 'vite-plugin-optimizer'
import { getReplacer } from './scripts/plugins/optimizer'
import { devPlugin, buildPlugin } from './scripts/plugins'
// https://vitejs.dev/config/
export default defineConfig({
  build: {
    rollupOptions: {
      plugins: [buildPlugin()],
    },
  },
  plugins: [
    Pages({
      dirs: 'src/pages',
      extensions: ['vue', 'md', 'tsx'],
    }),
    optimizer(getReplacer()),
    devPlugin(),
    vue({
      include: [/\.vue$/, /\.md$/],
    }),
    UnoCSS(),
    VueI18n({
      runtimeOnly: true,
      compositionOnly: true,
      fullInstall: true,
      include: [resolve(__dirname, './src/locales/**')],
    }),

    vueJsx(),
    Markdown({
      wrapperClasses: 'prose prose-sm m-auto text-left',
      headEnabled: true,
      markdownItSetup(md) {
        // https://prismjs.com/
        md.use(Shiki, {
          theme: {
            light: 'vitesse-light',
            dark: 'vitesse-dark',
          },
        })
        md.use(LinkAttributes, {
          matcher: (link: string) => /^https?:\/\//.test(link),
          attrs: {
            target: '_blank',
            rel: 'noopener',
          },
        })
      },
    }),
  ],
  resolve: {
    alias: {
      '@': '/src',
      '^': '/federal',
    },
  },
})
