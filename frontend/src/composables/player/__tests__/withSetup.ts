import { type App, createApp, defineComponent, h } from "vue";

/** Runs a composable inside a mounted component so lifecycle hooks work. */
export function withSetup<T>(setup: () => T): { result: T; app: App } {
  let result!: T;
  const app = createApp(
    defineComponent({
      setup() {
        result = setup();
        return () => h("div");
      },
    }),
  );
  app.mount(document.createElement("div"));
  return { result, app };
}
