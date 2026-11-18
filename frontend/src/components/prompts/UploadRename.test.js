import { createApp } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";

const { prompt } = vi.hoisted(() => ({
  prompt: { name: "upload-rename", confirm: () => {} },
}));

vi.mock("@/store", () => ({
  getters: { currentPrompt: () => prompt },
}));

import UploadRename from "./UploadRename.vue";

let app, root;

async function mount(folderName) {
  prompt.confirm = vi.fn();
  prompt.name = "upload-rename";
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(UploadRename, { initialName: folderName });
  app.directive("focus", {});
  app.config.globalProperties.$t = (key) => key;
  app.mount(root);
  await new Promise((resolve) => setTimeout(resolve, 0));
  return root;
}

describe("UploadRename", () => {
  afterEach(() => {
    app?.unmount();
    root?.remove();
  });

  it("starts with the conflicting folder name so the user can edit it", async () => {
    const el = await mount("upload-test-10");
    expect(el.querySelector("input").value).toBe("upload-test-10");
  });

  it("returns the typed name without touching anything on the server", async () => {
    const el = await mount("upload-test-10");
    const input = el.querySelector("input");
    input.value = "upload-test-101";
    input.dispatchEvent(new Event("input"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    el.querySelector('button[aria-label="general.rename"]').click();
    expect(prompt.confirm).toHaveBeenCalledWith(undefined, "upload-test-101");
  });

  it("cancels with no name", async () => {
    const el = await mount("upload-test-10");
    el.querySelector('button[aria-label="general.cancel"]').click();
    expect(prompt.confirm).toHaveBeenCalledWith(expect.anything(), null);
  });
});
