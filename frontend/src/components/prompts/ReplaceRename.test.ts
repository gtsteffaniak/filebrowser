// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { createApp, nextTick } from "vue";

const store = vi.hoisted(() => {
  const confirm = vi.fn();
  return {
    confirm,
    getters: {
      currentPrompt: () => ({ confirm, props: { allowSkip: true } }),
      sourcePermissions: () => ({ modify: true }),
      isShare: () => false,
    },
    state: { shareInfo: {} },
  };
});
vi.mock("@/store/index.ts", () => store);

import ReplaceRename from "./ReplaceRename.vue";

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.replaceChildren();
  store.confirm.mockReset();
});

it("submits only one conflict action while confirmation is pending", async () => {
  const { promise, resolve } = Promise.withResolvers<void>();
  store.confirm.mockReturnValue(promise);

  const root = document.createElement("div");
  document.body.append(root);
  app = createApp(ReplaceRename, { promptId: 1 });
  app.config.globalProperties.$t = (key: string) => key;
  app.mount(root);
  const overwrite = root.querySelector<HTMLButtonElement>(".button--red");
  const rename = root.querySelector<HTMLButtonElement>(".button--blue");
  if (!overwrite || !rename) throw new Error("Missing conflict actions");

  overwrite.click();
  overwrite.click();
  rename.click();
  await nextTick();

  expect(store.confirm).toHaveBeenCalledTimes(1);
  expect(store.confirm).toHaveBeenCalledWith(expect.any(MouseEvent), "overwrite");
  expect([...root.querySelectorAll("button")].every((button) => button.disabled)).toBe(true);

  resolve();
  await promise;
  await nextTick();
  expect(overwrite.disabled).toBe(false);
});

