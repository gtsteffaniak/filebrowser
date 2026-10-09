import { createApp, nextTick } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
const { unarchive, mutations } = vi.hoisted(() => ({
  unarchive: vi.fn(),
  mutations: { closeTopPrompt: vi.fn(), setReload: vi.fn(), updateCurrentUser: vi.fn() },
}));
vi.mock("@/api", () => ({ resourcesApi: { unarchive } }));
vi.mock("@/store", () => ({ state: { user: { deleteAfterArchive: true } }, mutations, getters: { canCreateInSource: () => true, isShare: () => false } }));
vi.mock("@/utils", () => ({ url: { removeLastDir: () => "" } }));
vi.mock("@/notify", () => ({ notify: { showSuccess: vi.fn() } }));
vi.mock("@/utils/notificationActions", () => ({ goToItemNotificationButton: vi.fn() }));
vi.mock("@/components/files/FileList.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/ToggleSwitch.vue", () => ({ default: { render: () => null } }));
import Unarchive from "./Unarchive.vue";

let app, root;
function mount(path = "/test.zip") {
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(Unarchive, { item: { path, source: "media" } });
  app.config.globalProperties.$t = (key) => key;
  app.mount(root);
  return root.querySelector('button[aria-label="prompts.unarchive"]');
}
const candidates = [
  { encoding: "cp932", names: ["テスト/ソ.txt"] },
  { encoding: "cp437", names: ["âeâXâg/â\\.txt"] },
];

describe("ZIP filename selection", () => {
  beforeEach(() => { unarchive.mockReset(); vi.clearAllMocks(); });
  afterEach(() => { app?.unmount(); root?.remove(); });
  it("preselects the suggestion, updates the preview and extracts with the user's choice", async () => {
    let resolve;
    unarchive.mockReturnValueOnce(new Promise((done) => { resolve = done; }));
    const extract = mount();
    expect(extract.disabled).toBe(true);
    expect(unarchive.mock.calls[0][0]).toMatchObject({ preview: true, destination: "/" });
    expect(unarchive.mock.calls[0][0]).not.toHaveProperty("deleteAfter");
    resolve({ suggested: "cp932", candidates });
    await vi.waitFor(() => expect(root.querySelector("select")?.value).toBe("cp932"));
    expect(root.querySelector(".filename-preview").textContent).toContain("テスト/ソ.txt");
    const select = root.querySelector("select");
    select.value = "cp437";
    select.dispatchEvent(new Event("change"));
    await nextTick();
    expect(root.querySelector(".filename-preview").textContent).toContain(candidates[1].names[0]);
    unarchive.mockResolvedValueOnce({});
    extract.click();
    await vi.waitFor(() => expect(unarchive).toHaveBeenCalledTimes(2));
    expect(unarchive.mock.calls[1][0]).toMatchObject({ filenameEncoding: "cp437", deleteAfter: true });
    expect(unarchive.mock.calls[1][0]).not.toHaveProperty("preview");
  });
  it("requires a choice when the encoding is ambiguous", async () => {
    unarchive.mockResolvedValue({ suggested: "", candidates });
    const extract = mount();
    await vi.waitFor(() => expect(root.querySelectorAll("select option")).toHaveLength(3));
    expect(extract.disabled).toBe(true);
    const select = root.querySelector("select");
    select.value = "cp932";
    select.dispatchEvent(new Event("change"));
    await nextTick();
    expect(extract.disabled).toBe(false);
  });
  it("blocks extraction on preview failure and allows retry", async () => {
    unarchive.mockRejectedValueOnce(new Error("unavailable"));
    const extract = mount();
    await vi.waitFor(() => expect(root.querySelector('[role="alert"]')?.textContent).toBe("prompts.zipEncodingError"));
    expect(extract.disabled).toBe(true);
    unarchive.mockResolvedValueOnce({ suggested: "cp932", candidates });
    root.querySelector(".encoding-options button").click();
    await vi.waitFor(() => expect(extract.disabled).toBe(false));
  });
  it("does not inspect tar archives", async () => {
    const extract = mount("/test.tar.gz");
    expect(unarchive).not.toHaveBeenCalled();
    expect(root.querySelector(".encoding-options")).toBeNull();
    expect(extract.disabled).toBe(false);
  });
});
