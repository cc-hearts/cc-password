import { defineConfig, presetAttributify, presetUno } from 'unocss'

export default defineConfig({
  presets: [
    presetAttributify({
      /* preset options */
    }),
    presetUno(),
    // ...custom presets
  ],
  rules: [
    [
      'border-ins',
      { 'border-color': 'var(--box-color-1)', transition: 'border 0.3s' },
    ],
    [
      'border-b-ins',
      {
        'border-bottom-color': 'var(--box-color-1)',
        transition: 'border 0.3s',
      },
    ],
    [
      'keyboard-bg',
      {
        'background-color': 'var(--docsearch-searchbox-background)',
      },
    ],
    ['app-drag', { '-webkit-app-region': 'drag' }],
    ['app-no-drag', { '-webkit-app-region': 'no-drag' }],
  ],
})
