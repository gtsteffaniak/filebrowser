import { getObjectProperty } from "@/utils/object.js";
import { KIND_ORDER, getKindKey } from "@/utils/mimetype.js";
import type { FileListItem } from "@/store/types.ts";

type SortableItem = Partial<FileListItem>;
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

export function sortedItems<T extends SortableItem>(items: T[] = [], sortby = "name", asc = true): T[] {
  return items.sort((a, b) => {
    const aPinned = !!a.pinned;
    const bPinned = !!b.pinned;

    if (aPinned !== bPinned) {
      return aPinned ? -1 : 1;
    }

    let valueA = getObjectProperty(a, sortby) as string | number;
    let valueB = getObjectProperty(b, sortby) as string | number;

    // Special handling for duration which is stored in metadata
    if (sortby === "duration") {
      valueA = (a.metadata as { duration?: number } | undefined)?.duration ?? 0;
      valueB = (b.metadata as { duration?: number } | undefined)?.duration ?? 0;
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
      return collator.compare(String(a.name ?? ""), String(b.name ?? ""));
    }

    if (sortby === "name") {
      // Use localeCompare with numeric option for natural sorting
      const left = String(valueA ?? "");
      const right = String(valueB ?? "");
      const comparison = collator.compare(left, right);
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
