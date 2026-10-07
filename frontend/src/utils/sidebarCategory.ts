// Sidebar link category helpers.
//
// Source link categories carry a display variant ("source", "source-minimal",
// "source-alt", "source-hybrid", "source-hybrid-2") and an optional "-root"
// suffix selecting the root-filesystem-only disk usage instead of the
// aggregated nested-mount usage reported by the backend
// (total/usedAlt vs totalRoot/usedAltRoot).

export const ROOT_ONLY_SUFFIX = "-root";

const SOURCE_CATEGORY_BASES = [
  "source",
  "source-minimal",
  "source-alt",
  "source-hybrid",
  "source-hybrid-2",
];

/** Strip the root-only "-root" suffix, returning the base category. */
export function baseSidebarCategory(category?: string): string {
  if (typeof category !== "string") return "";
  return category.endsWith(ROOT_ONLY_SUFFIX)
    ? category.slice(0, -ROOT_ONLY_SUFFIX.length)
    : category;
}

/** True when the category selects the root-filesystem-only disk usage variant. */
export function isRootOnlySidebarCategory(category?: string): boolean {
  return typeof category === "string" && category.endsWith(ROOT_ONLY_SUFFIX);
}

/** True when the category is any source-style sidebar link variant. */
export function isSourceSidebarCategory(category?: string): boolean {
  return SOURCE_CATEGORY_BASES.includes(baseSidebarCategory(category));
}

/** Apply or remove the root-only suffix on a source category. */
export function withRootOnlySuffix(category: string, rootOnly: boolean): string {
  const base = baseSidebarCategory(category);
  if (!rootOnly || base === "source-minimal" || !isSourceSidebarCategory(base)) {
    return base;
  }
  return base + ROOT_ONLY_SUFFIX;
}
