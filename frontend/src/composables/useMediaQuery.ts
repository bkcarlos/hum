import { onBeforeUnmount, onMounted, ref } from 'vue'

/** Reactive matchMedia. Used for the responsive dual-pane → tabs degradation. */
export function useMediaQuery(query: string) {
  const matches = ref(false)
  let mql: MediaQueryList | null = null
  const onChange = (e: MediaQueryListEvent) => (matches.value = e.matches)

  onMounted(() => {
    mql = window.matchMedia(query)
    matches.value = mql.matches
    mql.addEventListener('change', onChange)
  })
  onBeforeUnmount(() => mql?.removeEventListener('change', onChange))

  return matches
}
