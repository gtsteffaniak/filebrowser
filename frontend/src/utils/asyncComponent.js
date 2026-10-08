/** biome-ignore-all lint/correctness/noUnresolvedImports: false positives */
import { defineAsyncComponent } from 'vue';
import LoadingSpinner from '@/components/LoadingSpinner.vue';
import LoadFailed from '@/components/LoadFailed.vue';

export function createAsyncComponent(loader, timeout = 15000) {
  return defineAsyncComponent({
    loader,
    loadingComponent: LoadingSpinner,
    errorComponent: LoadFailed,
    timeout,
  });
}
