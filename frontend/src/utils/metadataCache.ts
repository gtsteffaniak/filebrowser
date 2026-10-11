// src/utils/metadataCache.ts
import { state } from "@/store/index.ts";

/** Which source (or share) a folder belongs to */
export interface DirMetadataScope {
  isShare?: boolean;
  source?: string;
  hash?: string;
}

export interface DirMetadataKey extends DirMetadataScope {
  path: string;
}

/** metadata by file name */
export type DirMetadataMap = Map<string, object>;

/** Per-item changes recorded since a folder was cached */
export interface PendingDirChanges {
  /** deleted or moved away */
  removed: Set<string>;
  /** uploaded, copied or moved */
  dirty: Set<string>;
  /** names without extension */
  stems: Set<string>;
}

export interface DirMetadataEntry extends PendingDirChanges {
  key: string;
  promise: Promise<DirMetadataMap>;
}

// maximum number of entries to prevent memory grow
const MAX_CACHE_ITEMS = 800;
const MAX_PENDING_CHANGES = 200;
// least recently used first
const cacheMap = new Map<string, DirMetadataEntry>();
let cacheUser: string | null = null;

// Entries are not keyed by user, the cache is dropped whenever the signed-in user logs out.
// one account's metadata is never served to the other.
function cache(): Map<string, DirMetadataEntry> {
  const user = state.user?.username || "";
  if (user !== cacheUser) {
    cacheMap.clear();
    cacheUser = user;
  }
  return cacheMap;
}

function scopePart({ isShare, source, hash }: DirMetadataScope): string {
  return isShare ? `share:${hash}` : `src:${source}`;
}

function normalizePath(path: string | undefined): string {
  const collapsed = `/${path || ""}`.replace(/\/+/g, "/");
  return collapsed.length > 1 ? collapsed.replace(/\/$/, "") : "/";
}

function getCacheKey({ isShare, source, hash, path }: DirMetadataKey): string {
  return `${scopePart({ isShare, source, hash })}|${normalizePath(path)}`;
}

function splitItemPath(itemPath: string | undefined): { dir: string; name: string; isDir: boolean } {
  const raw = itemPath || "";
  const full = normalizePath(raw);
  const idx = full.lastIndexOf("/");
  return {
    dir: idx <= 0 ? "/" : full.slice(0, idx),
    name: full.slice(idx + 1),
    isDir: raw.endsWith("/"),
  };
}

/** file name without its last extension, so "song.mp3" and "song.lrc" match */
export function stemOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return (dot > 0 ? name.slice(0, dot) : name).toLowerCase();
}

function dropOnReject(entry: DirMetadataEntry): void {
  entry.promise.catch(() => {
    if (cache().get(entry.key) === entry) {
      cache().delete(entry.key);
    }
  });
}

/** Looking an entry up counts as using it, so the folders you keep returning to are removed last */
export function getDirMetadataEntry(opts: DirMetadataKey): DirMetadataEntry | null {
  const entries = cache();
  const key = getCacheKey(opts);
  const entry = entries.get(key);
  if (!entry) {
    return null;
  }
  entries.delete(key);
  entries.set(key, entry);
  return entry;
}

/** Stores a pending or resolved promise for this folder. */
export function setCachedDirMetadata(opts: DirMetadataKey, promise: Promise<DirMetadataMap>): void {
  const entries = cache();
  const key = getCacheKey(opts);
  if (entries.has(key)) {
    entries.delete(key);
  } else if (entries.size >= MAX_CACHE_ITEMS) {
    const oldest = entries.keys().next().value;
    if (oldest !== undefined) {
      entries.delete(oldest);
    }
  }
  const entry: DirMetadataEntry = { key, promise, removed: new Set(), dirty: new Set(), stems: new Set() };
  entries.set(key, entry);
  dropOnReject(entry);
}

/**
 * Sends the per-item changes to the caller and starts clean.
 * @returns the changes, or null when are none
 */
export function takePendingDirChanges(entry: DirMetadataEntry): PendingDirChanges | null {
  if (entry.removed.size + entry.dirty.size + entry.stems.size === 0) {
    return null;
  }
  const pending = { removed: entry.removed, dirty: entry.dirty, stems: entry.stems };
  entry.removed = new Set();
  entry.dirty = new Set();
  entry.stems = new Set();
  return pending;
}

/** Replaces the promise of an existing entry (after pending changes were applied). */
export function setDirMetadataEntryPromise(entry: DirMetadataEntry, promise: Promise<DirMetadataMap>): void {
  entry.promise = promise;
  dropOnReject(entry);
}

function dropSubtree(scope: DirMetadataScope, itemPath: string): void {
  const entries = cache();
  const prefix = `${scopePart(scope)}|`;
  const root = normalizePath(itemPath);
  for (const key of [...entries.keys()]) {
    if (!key.startsWith(prefix)) {
      continue;
    }
    const pathPart = key.slice(prefix.length);
    if (pathPart === root || pathPart.startsWith(`${root}/`)) {
      entries.delete(key);
    }
  }
}

function recordItemChange(scope: DirMetadataScope, itemPath: string, kind: "removed" | "changed"): void {
  const { dir, name, isDir } = splitItemPath(itemPath);
  if (!name) {
    return;
  }
  dropSubtree(scope, itemPath);
  if (kind === "changed" && isDir) {
    return; // folders have no media metadata of their own
  }
  const key = getCacheKey({ ...scope, path: dir });
  const entry = cache().get(key);
  if (!entry) {
    return;
  }
  if (kind === "removed") {
    entry.removed.add(name);
    entry.dirty.delete(name);
  } else {
    entry.dirty.add(name);
    entry.removed.delete(name);
  }
  entry.stems.add(stemOf(name));
  if (entry.removed.size + entry.dirty.size > MAX_PENDING_CHANGES) {
    cache().delete(key);
  }
}

/** An item was deleted or moved: drop only its entry */
export function removeDirMetadataItem(scope: DirMetadataScope, itemPath: string): void {
  recordItemChange(scope, itemPath, "removed");
}

/** An item was uploaded, copied or moved: refresh only its entry */
export function markDirMetadataItemChanged(scope: DirMetadataScope, itemPath: string): void {
  recordItemChange(scope, itemPath, "changed");
}

/** drops its folder's listing and everything cached below the item */
export function invalidateDirMetadataItem(scope: DirMetadataScope, itemPath: string): void {
  dropSubtree(scope, itemPath);
  cache().delete(getCacheKey({ ...scope, path: splitItemPath(itemPath).dir }));
}

/**
 * Invalidates cached dir metadata so the next call gets fresh data: one folder with `path`, a whole
 * source or share without it, everything with no options.
 */
export function invalidateDirMetadataCache(opts: DirMetadataScope & { path?: string } = {}): void {
  const entries = cache();
  const { source, hash, path } = opts;
  const isShare = opts.isShare ?? (hash !== undefined && source === undefined);
  if (path !== undefined) {
    entries.delete(getCacheKey({ isShare, source, hash, path }));
    return;
  }
  if (source === undefined && hash === undefined) {
    entries.clear();
    return;
  }
  const prefix = `${scopePart({ isShare, source, hash })}|`;
  for (const key of [...entries.keys()]) {
    if (key.startsWith(prefix)) {
      entries.delete(key);
    }
  }
}
