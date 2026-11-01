import { afterEach, describe, expect, it, vi } from "vitest";
import { createApp, h } from "vue";

const { mockState } = vi.hoisted(() => ({ mockState: {} }));
vi.mock("@/store", () => ({ state: mockState, getters: { getTime: () => "" } }));
vi.mock("@/api", () => ({ resourcesApi: {} }));
vi.mock("qrcode.vue", () => ({ default: { render: () => null } }));
import FilesShareInfo from "./ShareInfo.vue";
import PromptShareInfo from "../prompts/ShareInfo.vue";

// Shape returned by GET /public/api/share/info: `bannerUrl` is set, `banner` is not.
const publicShareInfo = {
  hash: "abc",
  shareType: "normal",
  title: "Shared folder",
  bannerUrl: "https://example.com/logo.svg",
  shareURL: "https://example.com/public/share/abc",
};

function render(component, props = {}) {
  const el = document.createElement("div");
  const app = createApp({ render: () => h(component, props) });
  app.config.globalProperties.$t = (key) => key;
  app.mount(el);
  return { el, app };
}

describe("share banner for public visitors", () => {
  let mounted;
  afterEach(() => mounted?.app.unmount());

  it("sidebar share info shows the banner from bannerUrl", () => {
    Object.assign(mockState, { shareInfo: { ...publicShareInfo }, req: {} });
    mounted = render(FilesShareInfo, { hash: "abc", token: "" });
    const img = mounted.el.querySelector(".banner img");
    expect(img).not.toBeNull();
    expect(img.getAttribute("src")).toBe(publicShareInfo.bannerUrl);
  });

  it("share info prompt shows the banner from bannerUrl", () => {
    Object.assign(mockState, { shareInfo: { ...publicShareInfo }, req: {} });
    mounted = render(PromptShareInfo);
    const img = mounted.el.querySelector(".banner img");
    expect(img).not.toBeNull();
    expect(img.getAttribute("src")).toBe(publicShareInfo.bannerUrl);
  });

  it("no banner block when the share has no banner", () => {
    Object.assign(mockState, { shareInfo: { ...publicShareInfo, bannerUrl: "" }, req: {} });
    mounted = render(FilesShareInfo, { hash: "abc", token: "" });
    expect(mounted.el.querySelector(".banner")).toBeNull();
  });
});
