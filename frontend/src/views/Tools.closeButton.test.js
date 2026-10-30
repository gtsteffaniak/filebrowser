import { beforeEach, describe, expect, it, vi } from "vitest";

const { mutations } = vi.hoisted(() => ({
  mutations: { resetSelected: vi.fn() },
}));
vi.mock("@/store", () => ({ getters: {}, mutations }));
vi.mock("@/utils/constants", () => ({ tools: [] }));
vi.mock("@/utils/asyncComponent.js", () => ({ createAsyncComponent: () => ({}) }));
vi.mock("@/views/Errors.vue", () => ({ default: { render: () => null } }));
import Tools from "./Tools.vue";

const { methods } = Tools;

describe("Tools advanced search close button", () => {
  let $router;

  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState({}, "");
    $router = { go: vi.fn(), push: vi.fn(() => Promise.resolve()) };
  });

  it("goes back to the page the search was opened from", () => {
    window.history.replaceState({ back: "/files/data/photos/" }, "");
    methods.closeAdvancedSearch.call({ $router });
    expect($router.go).toHaveBeenCalledWith(-1);
    expect($router.push).not.toHaveBeenCalled();
  });

  it("opens the files when there is no previous page, e.g. the search URL was opened directly", () => {
    methods.closeAdvancedSearch.call({ $router });
    expect($router.go).not.toHaveBeenCalled();
    expect($router.push).toHaveBeenCalledWith({ path: "/files" });
  });

  it("drops the selection so it doesn't leak into the page it returns to", () => {
    methods.closeAdvancedSearch.call({ $router });
    expect(mutations.resetSelected).toHaveBeenCalled();
  });
});
