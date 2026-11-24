import { reactive, watch } from "vue";
import { utcStartOfDaySecondsFromDateInput } from "@/utils/moment.ts";

export interface SearchFilters {
  largerThan: string | number;
  smallerThan: string | number;
  modifiedOlderThan: string;
  modifiedNewerThan: string;
  entryTypeFilter: string;
  selectedMediaTypes: string[];
  useWildcardSearch: boolean;
  caseExactSearch: boolean;
}

const entryTypeValues = new Set(["type:file", "type:folder"]);
const mediaTypeValues = new Set(["type:image", "type:audio", "type:video", "type:doc", "type:archive"]);

function queryTruthy(v: unknown) {
  if (v === undefined || v === null || v === "") {
    return false;
  }
  if (v === true || v === 1) {
    return true;
  }
  const s = String(v).toLowerCase();
  return s === "1" || s === "true" || s === "yes";
}

function routeString(raw: unknown) {
  return raw !== undefined && raw !== null ? String(raw).trim() : "";
}

/**
 * Filter state shared by the quick search and the advanced search, plus the
 * conversions between the filters, the search request and the route query.
 * `onChange` runs whenever a filter that affects the results changes.
 */
export function useSearchFilters(options: { onChange?: () => void } = {}) {
  const filters = reactive<SearchFilters>({
    largerThan: "",
    smallerThan: "",
    modifiedOlderThan: "",
    modifiedNewerThan: "",
    entryTypeFilter: "all",
    selectedMediaTypes: [],
    useWildcardSearch: false,
    caseExactSearch: false,
  });

  const notify = () => options.onChange?.();

  watch(
    () => [
      filters.largerThan,
      filters.smallerThan,
      filters.modifiedOlderThan,
      filters.modifiedNewerThan,
      filters.useWildcardSearch,
      filters.caseExactSearch,
    ],
    notify,
  );
  watch(
    () => filters.entryTypeFilter,
    (newValue) => {
      if (newValue === "type:folder" && filters.selectedMediaTypes.length > 0) {
        // media types don't apply to folders; clearing them triggers the change itself
        filters.selectedMediaTypes = [];
        return;
      }
      notify();
    },
  );
  watch(() => filters.selectedMediaTypes, notify, { deep: true });

  /** `type:...` tokens for the selected entry/media types, with a trailing space (or ""). */
  function typesPrefix() {
    const parts: string[] = [];
    if (filters.entryTypeFilter !== "all") {
      parts.push(filters.entryTypeFilter);
    }
    parts.push(...filters.selectedMediaTypes);
    return parts.length > 0 ? `${parts.join(" ")} ` : "";
  }

  /** What `toolsApi.search` needs: the query prefix (everything but the search term) and the extra params. */
  function buildRequest() {
    let prefix = typesPrefix();
    if (filters.largerThan !== "") {
      prefix = `${prefix}type:largerThan=${filters.largerThan} `;
    }
    if (filters.smallerThan !== "") {
      prefix = `${prefix}type:smallerThan=${filters.smallerThan} `;
    }
    const dateParams: { olderThan?: number; newerThan?: number; useWildcard?: boolean } = {};
    const olderUnix = utcStartOfDaySecondsFromDateInput(filters.modifiedOlderThan);
    if (olderUnix !== null) {
      dateParams.olderThan = olderUnix;
    }
    const newerUnix = utcStartOfDaySecondsFromDateInput(filters.modifiedNewerThan);
    if (newerUnix !== null) {
      dateParams.newerThan = newerUnix;
    }
    if (filters.useWildcardSearch) {
      dateParams.useWildcard = true;
    }
    const queryPrefix = filters.caseExactSearch ? `case:exact ${prefix}` : prefix;
    return { queryPrefix, dateParams };
  }

  /** Route query params for the filters (the advanced search page reads the same keys back). */
  function toRouteQuery() {
    const query: Record<string, string> = {};
    const types = typesPrefix().trim();
    if (types !== "") {
      query.types = types;
    }
    if (filters.largerThan !== "") {
      query.largerThan = String(filters.largerThan);
    }
    if (filters.smallerThan !== "") {
      query.smallerThan = String(filters.smallerThan);
    }
    if (filters.modifiedOlderThan !== "") {
      query.dateOlder = filters.modifiedOlderThan;
    }
    if (filters.modifiedNewerThan !== "") {
      query.dateNewer = filters.modifiedNewerThan;
    }
    if (filters.useWildcardSearch) {
      query.wildcard = "1";
    }
    if (filters.caseExactSearch) {
      query.caseExact = "1";
    }
    if (filters.entryTypeFilter === "type:folder") {
      query.typeLock = "1";
    }
    return query;
  }

  /** Inverse of `toRouteQuery`. */
  function applyRouteQuery(q: Record<string, unknown>) {
    let entryTypeFilter = "all";
    let selectedMediaTypes: string[] = [];
    for (const token of routeString(q.types).split(/\s+/)) {
      if (entryTypeValues.has(token)) {
        entryTypeFilter = token;
      } else if (mediaTypeValues.has(token)) {
        selectedMediaTypes.push(token);
      }
    }
    if (queryTruthy(q.typeLock)) {
      entryTypeFilter = "type:folder";
      selectedMediaTypes = [];
    }
    Object.assign(filters, {
      entryTypeFilter,
      selectedMediaTypes,
      largerThan: routeString(q.largerThan),
      smallerThan: routeString(q.smallerThan),
      modifiedOlderThan: routeString(q.dateOlder),
      modifiedNewerThan: routeString(q.dateNewer),
      useWildcardSearch: queryTruthy(q.wildcard),
      caseExactSearch: queryTruthy(q.caseExact),
    });
  }

  /** Whether a route query sets any of the filters. */
  function hasRouteFilters(q: Record<string, unknown>) {
    return (
      ["types", "largerThan", "smallerThan", "dateOlder", "dateNewer"].some((key) => routeString(q[key]) !== "") ||
      ["wildcard", "caseExact", "typeLock"].some((key) => queryTruthy(q[key]))
    );
  }

  return { filters, buildRequest, toRouteQuery, applyRouteQuery, hasRouteFilters };
}
