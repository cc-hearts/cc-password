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
      { 'border-color': 'var(--box-color-1)', transform: 'border 0.3s' },
    ],
    [
      'keyboard-bg',
      {
        'background-color': 'var(--docsearch-searchbox-background)',
      },
    ],
  ],
})
