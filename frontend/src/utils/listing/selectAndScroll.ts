import { mutations, state } from "@/store/index.ts";
import { base64Encode } from "@/utils/url.js";
import { getVirtualListing } from "@/utils/listing/virtualListing.ts";

/** Selects the item at `index` in the current listing and scrolls it into view. False when its element isn't rendered yet. */
export function selectAndScrollToIndex(index: number, replaceSelection = false): boolean {
  const item = state.req.items.at(index);
  if (item === undefined) return false;
  if (replaceSelection) {
    mutations.resetSelected();
  }
  mutations.addSelected(index);
  const scrollOptions = { behavior: "instant", block: "center" } as const;
  const listing = getVirtualListing();
  if (listing) return listing.scrollToIndex(index, scrollOptions);
  const el = document.querySelector(
    `#${CSS.escape(base64Encode(encodeURIComponent(`${item.source}::${item.path}`)))}`,
  );
  if (!el) return false;
  el.scrollIntoView(scrollOptions);
  return true;
}
