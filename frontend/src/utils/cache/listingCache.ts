import { state } from "@/store/index.ts";
import type { ReqObject } from "@/store/types.ts";

interface ListingCacheEntry {
  res: ReqObject;
  time: number;
  items: number;
}

const TTL_MS = 120000;
const MAX_ENTRIES = 6; // list maximum
const MAX_TOTAL_ITEMS = 10000; // maximum items in total
const MAX_ENTRY_ITEMS = 5000; // folders with more items than this are always fetched (so no cache at all)

// least recently used first
const cache = new Map<string, ListingCacheEntry>();
let totalItems = 0;
let cacheUser: string | null = null;

// the previous account's listings are dropped when the signed-in user changes
function checkUser(): void {
  const user = state.user?.username || "";
  if (user !== cacheUser) {
    clearListingCache();
    cacheUser = user;
  }
}

function trimTrailingSlash(p: string | undefined): string {
  return (p || "/").replace(/\/+$/, "") || "/";
}

export function listingCacheKey(source: string, path: string): string {
  return `${state.user?.username}\n${source}\n${trimTrailingSlash(path)}`;
}

export function deleteCachedListing(key: string): void {
  const entry = cache.get(key);
  if (entry) {
    totalItems -= entry.items;
    cache.delete(key);
  }
}

/** Looking an entry up counts as using it, so the folders you keep returning to are removed last */
export function getCachedListing(key: string): ListingCacheEntry | null {
  checkUser();
  const entry = cache.get(key);
  if (!entry) {
    return null;
  }
  if (Date.now() - entry.time > TTL_MS) {
    deleteCachedListing(key);
    return null;
  }
  cache.delete(key);
  cache.set(key, entry);
  return entry;
}

function dropExpired(): void {
  const now = Date.now();
  for (const [key, entry] of cache) {
    if (now - entry.time > TTL_MS) {
      deleteCachedListing(key);
    }
  }
}

export function cacheListing(key: string, res: ReqObject): void {
  checkUser();
  deleteCachedListing(key);
  dropExpired();
  const items = res.items?.length ?? 0;
  if (items > MAX_ENTRY_ITEMS) {
    return;
  }
  cache.set(key, { res, time: Date.now(), items });
  totalItems += items;
  while (cache.size > MAX_ENTRIES || totalItems > MAX_TOTAL_ITEMS) {
    const oldest = cache.keys().next().value;
    if (oldest === undefined) {
      break;
    }
    deleteCachedListing(oldest);
  }
}

export function clearListingCache(): void {
  cache.clear();
  totalItems = 0;
}
