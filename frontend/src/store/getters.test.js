import { beforeEach, describe, expect, it } from "vitest";
// Import the real getters implementation directly (the global test setup mocks the
// "@/store" barrel, so `getters` is not exported from there). getters.js binds to
// the same mocked `state` singleton we mutate below.
import { getters } from "@/store/getters.js";
import { state } from "@/store";

// Regression: a PDF must open in the PDF preview (iframe), never the text editor,
// even when the backend included a `content` field for it. Some ASCII-heavy PDFs
// pass the backend's text byte-heuristic and would otherwise be misrouted to the
// editor by the generic `'content' in req -> editor` fallback.
describe("getters.currentView PDF routing", () => {
  beforeEach(() => {
    state.navigation = { isTransitioning: false };
    state.user = { username: "tester", disableViewingExt: "" };
    state.route = { path: "/files/report.pdf" };
  });

  it("routes a PDF that has a content field to the preview, not the editor", () => {
    state.req = {
      type: "application/pdf",
      name: "report.pdf",
      content: "%PDF-1.4 catalog pages ...",
    };
    expect(getters.currentView()).toBe("preview");
  });

  it("routes a PDF without a content field to the preview", () => {
    state.req = { type: "application/pdf", name: "report.pdf" };
    expect(getters.currentView()).toBe("preview");
  });

  it("still routes a genuine text file with content to the editor", () => {
    state.req = { type: "text/plain", name: "notes.txt", content: "hello" };
    expect(getters.currentView()).toBe("editor");
  });
});
