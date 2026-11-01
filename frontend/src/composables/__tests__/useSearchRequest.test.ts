import { afterEach, describe, expect, it, vi } from "vitest";
import { createApp, defineComponent, h, type App } from "vue";
import { useSearchRequest } from "../useSearchRequest.ts";

let app: App | null = null;
afterEach(() => {
  app?.unmount();
  app = null;
});

function mountComposable() {
  let api!: ReturnType<typeof useSearchRequest>;
  app = createApp(
    defineComponent({
      setup() {
        api = useSearchRequest();
        return () => h("div");
      },
    }),
  );
  app.mount(document.createElement("div"));
  return { api, unmount: () => app?.unmount() };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useSearchRequest", () => {
  it("applies the response while tracking the running state", async () => {
    const { api } = mountComposable();
    const apply = vi.fn();
    const pending = deferred<string>();
    const done = api.run(() => pending.promise, apply);
    expect(api.isRunning.value).toBe(true);
    pending.resolve("found");
    await expect(done).resolves.toBe(true);
    expect(apply).toHaveBeenCalledWith("found");
    expect(api.isRunning.value).toBe(false);
  });

  it("aborts a superseded request and ignores its late response", async () => {
    const { api } = mountComposable();
    const first = deferred<string>();
    const second = deferred<string>();
    let firstSignal!: AbortSignal;
    const applyFirst = vi.fn();
    const applySecond = vi.fn();

    const firstDone = api.run((signal) => {
      firstSignal = signal;
      return first.promise;
    }, applyFirst);
    const secondDone = api.run(() => second.promise, applySecond);
    expect(firstSignal.aborted).toBe(true);

    // the old request settling must not end the running state of the new one
    first.resolve("old");
    await expect(firstDone).resolves.toBe(false);
    expect(api.isRunning.value).toBe(true);

    second.resolve("new");
    await expect(secondDone).resolves.toBe(true);
    expect(applyFirst).not.toHaveBeenCalled();
    expect(applySecond).toHaveBeenCalledWith("new");
    expect(api.isRunning.value).toBe(false);
  });

  it("only reports errors of the latest request", async () => {
    const { api } = mountComposable();
    const first = deferred<string>();
    const second = deferred<string>();
    const onError = vi.fn();
    const firstDone = api.run(() => first.promise, vi.fn(), onError);
    const secondDone = api.run(() => second.promise, vi.fn(), onError);

    first.reject(new DOMException("aborted", "AbortError"));
    await firstDone;
    expect(onError).not.toHaveBeenCalled();

    const failure = new Error("boom");
    second.reject(failure);
    await expect(secondDone).resolves.toBe(false);
    expect(onError).toHaveBeenCalledExactlyOnceWith(failure);
  });

  it("abort() and unmounting drop the in-flight request", async () => {
    const { api, unmount } = mountComposable();
    const pending = deferred<string>();
    const apply = vi.fn();
    let signal!: AbortSignal;
    const done = api.run((s) => {
      signal = s;
      return pending.promise;
    }, apply);

    api.abort();
    expect(signal.aborted).toBe(true);
    expect(api.isRunning.value).toBe(false);
    pending.resolve("late");
    await expect(done).resolves.toBe(false);
    expect(apply).not.toHaveBeenCalled();

    const next = deferred<string>();
    let nextSignal!: AbortSignal;
    const nextDone = api.run((s) => {
      nextSignal = s;
      return next.promise;
    }, vi.fn());
    unmount();
    expect(nextSignal.aborted).toBe(true);
    next.resolve("x");
    await nextDone;
  });
});
