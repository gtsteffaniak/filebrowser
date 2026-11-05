import { afterEach, describe, expect, it, vi } from "vitest";
import { createApp, h, nextTick } from "vue";

vi.mock("@/store", () => ({
  getters: { isDarkMode: () => false },
  state: { sessionId: "test-session" },
}));

import TextPreview from "./TextPreview.vue";
import { clearTextPreviewCache } from "@/utils/textPreview.ts";

function payload(overrides: Record<string, unknown> = {}) {
  return {
    content: "",
    kind: "plain",
    format: "text",
    truncated: false,
    size: 1,
    ...overrides,
  };
}

function stubFetch(data: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify(data), { status: 200 })),
  );
}

async function flush() {
  // Wait for the fetch promise chain and queued limiter to settle.
  await new Promise((resolve) => setTimeout(resolve, 0));
  await nextTick();
  await nextTick();
}

function mount(props: { url: string; modified?: string | null; language?: string | null }) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const app = createApp({ render: () => h(TextPreview, props) });
  app.mount(el);
  return { el, app };
}

describe("TextPreview rendering", () => {
  afterEach(() => {
    clearTextPreviewCache();
    vi.unstubAllGlobals();
    document.body.innerHTML = "";
  });

  it("does not render a truncation fade for a truncated payload", async () => {
    stubFetch(payload({ content: "<p>hello world</p>", kind: "html", format: "html", truncated: true, size: 99_999 }));
    const { el, app } = mount({ url: "/api/resources/preview/text?path=%2Fa.html", modified: "1" });
    await flush();

    expect(el.querySelector(".text-preview-html")?.textContent).toContain("hello world");
    expect(el.querySelector(".text-preview-fade")).toBeNull();
    app.unmount();
  });

  it("uses the language prop to pick the CSV delimiter", async () => {
    stubFetch(payload({ content: "a\tb\n1\t2", kind: "csv" }));
    const { el, app } = mount({
      url: "/api/resources/preview/text?path=%2Fdata.tsv",
      modified: "1",
      language: "tsv",
    });
    await flush();

    const cells = el.querySelectorAll(".text-preview-table thead th");
    expect(cells).toHaveLength(2);
    expect(cells[0]?.textContent).toBe("a");
    expect(cells[1]?.textContent).toBe("b");
    app.unmount();
  });

  it("reuses the rendered HTML cache for the same url and modified time", async () => {
    stubFetch(payload({ content: "<p>cached</p>", kind: "html", format: "html" }));
    const props = { url: "/api/resources/preview/text?path=%2Fb.html", modified: "7" };

    const first = mount(props);
    await flush();
    expect(first.el.querySelector(".text-preview-html")?.textContent).toContain("cached");
    first.app.unmount();

    // A second mount (as happens when the popup reopens) must not re-fetch.
    const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;
    fetchMock.mockClear();
    const second = mount(props);
    await flush();
    expect(second.el.querySelector(".text-preview-html")?.textContent).toContain("cached");
    expect(fetchMock).not.toHaveBeenCalled();
    second.app.unmount();
  });
});
