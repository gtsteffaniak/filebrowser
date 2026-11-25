import { onBeforeUnmount, onMounted, ref } from "vue";

const fullscreenHost = ref<HTMLElement | null>(null);
let consumers = 0;

function syncFullscreen(): void {
  const el = document.fullscreenElement as HTMLElement | null;
  fullscreenHost.value = el && el !== document.documentElement && el !== document.body ? el : null;
}

export function useFullscreen() {
  onMounted(() => {
    if (consumers++ === 0) document.addEventListener("fullscreenchange", syncFullscreen);
    syncFullscreen();
  });
  onBeforeUnmount(() => {
    if (--consumers === 0) document.removeEventListener("fullscreenchange", syncFullscreen);
  });
  return fullscreenHost;
}
