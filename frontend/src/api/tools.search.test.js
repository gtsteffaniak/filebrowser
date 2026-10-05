import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/notify", () => ({ notify: { showError: vi.fn() } }));
vi.mock("@/utils/url.js", () => ({
  getApiPath: (_path, params) => JSON.stringify(params),
}));
vi.mock("./utils", () => ({ fetchURL: vi.fn(), fetchJSON: vi.fn() }));

import { fetchURL } from "./utils";
import { search } from "./tools";

describe("search result limit", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchURL.mockResolvedValue({ json: async () => [] });
  });

  it("passes the selected limit with repeated scopes and terms", async () => {
    await search(null, ["one", "two"], "", false, {
      limit: 750,
      terms: ["document"],
      perSourceScopes: [{ source: "one", path: "/docs" }, { source: "two", path: "/" }],
    });
    expect(JSON.parse(fetchURL.mock.calls[0][0])).toMatchObject({
      limit: "750", terms: ["document"], scope: ["one:/docs", "two:/"],
    });
  });

  it("leaves quick search at the server default", async () => {
    await search("/", "one", "document");
    expect(JSON.parse(fetchURL.mock.calls[0][0])).not.toHaveProperty("limit");
  });

  it("preserves the size viewer request", async () => {
    await search("/", "one", "", true);
    expect(JSON.parse(fetchURL.mock.calls[0][0])).toMatchObject({ largest: "true" });
    expect(JSON.parse(fetchURL.mock.calls[0][0])).not.toHaveProperty("limit");
  });
});
