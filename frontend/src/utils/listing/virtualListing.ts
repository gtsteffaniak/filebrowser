import type { HitRect } from "@/utils/listing/listingGrid.ts";

export interface VirtualScrollOptions {
  block?: ScrollLogicalPosition;
  behavior?: ScrollBehavior;
}

export interface VirtualListingHandle {
  /** False for short listings that render everything, there the DOM has all the answers */
  isActive(): boolean;
  /** Brings an item into view even when it is not rendered */
  scrollToIndex(index: number, options?: VirtualScrollOptions): boolean;
  /** The item an up/down arrow goes to from `index`, null when there is none */
  moveVertical(index: number, direction: -1 | 1): number | null;
  /** Items overlapping a rectangle given relative to the listing element, null when the DOM should be used */
  hitTest(rect: HitRect, listingRect: DOMRect): number[] | null;
}

let current: VirtualListingHandle | null = null;

export function registerVirtualListing(handle: VirtualListingHandle): () => void {
  current = handle;
  return () => {
    if (current === handle) current = null;
  };
}

export function getVirtualListing(): VirtualListingHandle | null {
  return current;
}
