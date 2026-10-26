import { mutations, state } from "@/store/index.ts";
import { base64Encode } from "@/utils/url.js";

/** Selects the item at `index` in the current listing and scrolls it into view. False when its element isn't rendered yet. */
export function selectAndScrollToIndex(index: number, replaceSelection = false): boolean {
  const item = state.req.items.at(index);
  if (item === undefined) return false;
  if (replaceSelection) {
    mutations.resetSelected();
  }
  mutations.addSelected(index);
  const el = document.getElementById(
    base64Encode(encodeURIComponent(`${item.source}::${item.path}`)),
  );
  if (!el) return false;
  el.scrollIntoView({ behavior: "instant", block: "center" });
  return true;
}
