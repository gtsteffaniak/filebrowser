import { getObjectProperty } from '@/utils/object';
import { KIND_ORDER, getKindKey } from '@/utils/mimetype';

export function sortedItems(items = [], sortby = "name", asc = true) {
  return items.sort((a, b) => {
    const aPinned = !!a.pinned;
    const bPinned = !!b.pinned;

    if (aPinned !== bPinned) {
      return aPinned ? -1 : 1;
    }

    let valueA = getObjectProperty(a, sortby);
    let valueB = getObjectProperty(b, sortby);

    // Special handling for duration which is stored in metadata
    if (sortby === "duration") {
      valueA = a.metadata?.duration ?? 0;
      valueB = b.metadata?.duration ?? 0;
    }

    if (sortby === "created") {
      const timeA = Date.parse(a.created ?? "") || 0;
      const timeB = Date.parse(b.created ?? "") || 0;
      if (timeA === timeB) {
        return 0;
      }
      return asc ? timeA - timeB : timeB - timeA;
    }

    if (sortby === "kind") {
      const rankDiff = KIND_ORDER.indexOf(getKindKey(a.type)) - KIND_ORDER.indexOf(getKindKey(b.type));
      if (rankDiff !== 0) {
        return asc ? rankDiff : -rankDiff;
      }
      return String(a.name ?? "").localeCompare(String(b.name ?? ""), undefined, { numeric: true, sensitivity: "base" });
    }

    if (sortby === "name") {
      // Use localeCompare with numeric option for natural sorting
      const left = String(valueA ?? "");
      const right = String(valueB ?? "");
      const comparison = left.localeCompare(right, undefined, { numeric: true, sensitivity: "base" });
      return asc ? comparison : -comparison;
    }

    if (valueA === valueB) {
      return 0;
    }

    // Default sorting for other fields
    if (asc) {
      return valueA > valueB ? 1 : -1;
    }
    return valueA < valueB ? 1 : -1;
  });
}
