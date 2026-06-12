import { getObjectProperty } from '@/utils/object';

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
