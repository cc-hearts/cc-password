import { defineComponent, watchEffect } from "vue";

export default defineComponent({
  name: 'LazyRender',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },

  setup(props, { slots }) {
    let isOnceRender = false
    watchEffect(() => {
      if (props.show && isOnceRender === false) {
        isOnceRender = true
      }
    })
    function render() {
      if (isOnceRender) {
        return <div style={{ display: 'contents' }} v-show={props.show}>
          {slots.default?.()}
        </div>
      }
      return null
    }

    return render
  }
})