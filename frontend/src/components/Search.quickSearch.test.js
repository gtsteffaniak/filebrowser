import { afterEach, describe, expect, it, vi } from "vitest";
import Search from "./Search.vue";

describe("quick search debounce", () => {
  afterEach(() => vi.useRealTimers());

  it("cancels the previous request and waits until typing pauses", () => {
    vi.useFakeTimers();
    const controller = new AbortController();
    const abort = vi.spyOn(controller, "abort");
    const search = {
      cancelSearch: Search.methods.cancelSearch,
      searchController: controller,
      searchRequestId: 0,
      searchTimeout: undefined,
      ongoing: 1,
      results: [{ path: "/old" }],
      noneMessage: "No results",
      submit: vi.fn().mockResolvedValue(undefined),
    };

    Search.methods.scheduleSearch.call(search);
    expect(abort).toHaveBeenCalledOnce();
    expect(search.results).toEqual([]);

    vi.advanceTimersByTime(250);
    Search.methods.scheduleSearch.call(search);
    vi.advanceTimersByTime(299);
    expect(search.submit).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);

    expect(search.submit).toHaveBeenCalledOnce();
  });

  it("handles a rejected request from the debounce timer", async () => {
    vi.useFakeTimers();
    const search = {
      cancelSearch: Search.methods.cancelSearch,
      searchTimeout: undefined,
      searchRequestId: 0,
      ongoing: 0,
      submit: vi.fn().mockRejectedValue(new Error("network failure")),
    };

    Search.methods.scheduleSearch.call(search);
    await vi.runAllTimersAsync();

    expect(search.submit).toHaveBeenCalledOnce();
  });
});
