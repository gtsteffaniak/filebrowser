import { describe, expect, it, vi } from "vitest";
import { effectScope, nextTick } from "vue";
import { useSearchFilters } from "../useSearchFilters.ts";

function setup() {
  const onChange = vi.fn();
  const scope = effectScope();
  const search = scope.run(() => useSearchFilters({ onChange })) as ReturnType<typeof useSearchFilters>;
  return { ...search, onChange, scope };
}

describe("useSearchFilters", () => {
  it.each([
    ["largerThan", 5],
    ["smallerThan", 5],
    ["modifiedOlderThan", "2024-01-01"],
    ["modifiedNewerThan", "2024-01-01"],
    ["useWildcardSearch", true],
    ["caseExactSearch", true],
    ["entryTypeFilter", "type:file"],
  ] as const)("notifies when %s changes", async (key, value) => {
    const { filters, onChange, scope } = setup();
    (filters as Record<string, unknown>)[key] = value;
    await nextTick();
    expect(onChange).toHaveBeenCalledTimes(1);
    scope.stop();
  });

  it("notifies when a media type is picked", async () => {
    const { filters, onChange, scope } = setup();
    filters.selectedMediaTypes.push("type:image");
    await nextTick();
    expect(onChange).toHaveBeenCalledTimes(1);
    scope.stop();
  });

  it("clears the media types when only folders are searched, notifying once", async () => {
    const { filters, onChange, scope } = setup();
    filters.selectedMediaTypes = ["type:image"];
    await nextTick();
    onChange.mockClear();

    filters.entryTypeFilter = "type:folder";
    await nextTick();
    await nextTick();
    expect(filters.selectedMediaTypes).toEqual([]);
    expect(onChange).toHaveBeenCalledTimes(1);
    scope.stop();
  });

  it("builds the request prefix and params from the filters", () => {
    const { filters, buildRequest, scope } = setup();
    filters.entryTypeFilter = "type:file";
    filters.selectedMediaTypes = ["type:image", "type:video"];
    filters.largerThan = 5;
    filters.useWildcardSearch = true;
    expect(buildRequest()).toEqual({
      queryPrefix: "type:file type:image type:video type:largerThan=5 ",
      dateParams: { useWildcard: true },
    });

    filters.caseExactSearch = true;
    expect(buildRequest().queryPrefix.startsWith("case:exact type:file")).toBe(true);
    scope.stop();
  });

  it("round-trips through the route query", () => {
    const { filters, toRouteQuery, scope } = setup();
    filters.entryTypeFilter = "type:file";
    filters.selectedMediaTypes = ["type:audio"];
    filters.smallerThan = 10;
    filters.modifiedOlderThan = "2024-01-01";
    filters.caseExactSearch = true;
    const query = toRouteQuery();
    expect(query).toEqual({
      types: "type:file type:audio",
      smallerThan: "10",
      dateOlder: "2024-01-01",
      caseExact: "1",
    });

    const other = setup();
    other.applyRouteQuery(query);
    expect(other.filters.entryTypeFilter).toBe("type:file");
    expect(other.filters.selectedMediaTypes).toEqual(["type:audio"]);
    expect(other.filters.smallerThan).toBe("10");
    expect(other.filters.caseExactSearch).toBe(true);
    other.scope.stop();
    scope.stop();
  });

  it("locks to folders when the route says so", () => {
    const { filters, applyRouteQuery, toRouteQuery, scope } = setup();
    applyRouteQuery({ types: "type:image", typeLock: "1" });
    expect(filters.entryTypeFilter).toBe("type:folder");
    expect(filters.selectedMediaTypes).toEqual([]);
    expect(toRouteQuery().typeLock).toBe("1");
    scope.stop();
  });

  it("detects filters in a route query", () => {
    const { hasRouteFilters, scope } = setup();
    expect(hasRouteFilters({})).toBe(false);
    expect(hasRouteFilters({ largerThan: " " })).toBe(false);
    expect(hasRouteFilters({ dateNewer: "2024-01-01" })).toBe(true);
    expect(hasRouteFilters({ wildcard: "true" })).toBe(true);
    scope.stop();
  });
});
