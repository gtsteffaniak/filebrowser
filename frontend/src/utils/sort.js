import { removeTrailingSlash } from "@/utils/url.js";

export function sortedItems(items = [], sortby = "name", asc = true, pinnedPaths = []) {
    // Normalise pinned paths
    const normalisedPinnedPaths = (Array.isArray(pinnedPaths) ? pinnedPaths : []).map(p => removeTrailingSlash(p));
    const pinnedSet = new Set(normalisedPinnedPaths);

    return items.sort((a, b) => {
        const aPathNorm = a.path ? removeTrailingSlash(a.path) : '';
        const bPathNorm = b.path ? removeTrailingSlash(b.path) : '';
        const aPinned = pinnedSet.has(aPathNorm);
        const bPinned = pinnedSet.has(bPathNorm);

        if (aPinned !== bPinned) {
            return aPinned ? -1 : 1;
        }

        let valueA = a[sortby];
        let valueB = b[sortby];

        // Special handling for duration which is stored in metadata
        if (sortby === "duration") {
            valueA = a.metadata?.duration ?? 0;
            valueB = b.metadata?.duration ?? 0;
        }

        if (sortby === "name") {
            // Use localeCompare with numeric option for natural sorting
            const comparison = valueA.localeCompare(valueB, undefined, { numeric: true, sensitivity: "base" });
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
