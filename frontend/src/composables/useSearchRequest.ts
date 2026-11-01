import { onBeforeUnmount, ref } from "vue";

/**
 * Runs search requests so only the latest one counts, a superseded request is aborted
 * and a late response can't overwrite newer results.
 */
export function useSearchRequest() {
  const isRunning = ref(false);
  // The latest request; anything else is superseded.
  let current: AbortController | null = null;

  /** Drops the in-flight request: its response is ignored and its connection released */
  function abort() {
    current?.abort();
    current = null;
    isRunning.value = false;
  }

  async function run<T>(
    task: (signal: AbortSignal) => Promise<T>,
    apply: (value: T) => void,
    onError?: (error: unknown) => void,
  ) {
    abort();
    const controller = new AbortController();
    current = controller;
    isRunning.value = true;
    try {
      const value = await task(controller.signal);
      if (current !== controller) {
        return false;
      }
      apply(value);
      return true;
    } catch (error) {
      if (current === controller) {
        onError?.(error);
      }
      return false;
    } finally {
      if (current === controller) {
        current = null;
        isRunning.value = false;
      }
    }
  }

  onBeforeUnmount(abort);

  return { isRunning, run, abort };
}
