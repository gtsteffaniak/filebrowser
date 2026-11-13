import { state } from "@/store/index.ts";

/** Which source (or share) a path belongs to */
export interface MetadataScope {
  isShare?: boolean;
  source?: string;
  hash?: string;
}

/** metadata by file name */
export type DirMetadataMap = Map<string, object>;

type Change = "removed" | "changed";

interface Entry {
  scope: string;
  /** normalized path of the folder or file */
  path: string;
  promise: Promise<unknown>;
  time: number;
  /** items in a folder entry, they count against MAX_FOLDER_ITEMS */
  size: number;
  /** folder entries only: items deleted or uploaded since it was fetched */
  changes?: Map<string, Change>;
}

const TTL_MS = 600000;
const MAX_FILE_ENTRIES = 30;
const MAX_FOLDER_ITEMS = 5000;
const MAX_PENDING_CHANGES = 200;
const DIR = "dir";
// folders and single files, least recently used first
const cache = new Map<string, Entry>();
let cacheUser: string | null = null;

// Entries are not keyed by user, the cache is dropped whenever the signed-in user changes,
// one account's metadata is never served to the other.
function checkUser(): void {
  const user = state.user?.username || "";
  if (user !== cacheUser) {
    cache.clear();
    cacheUser = user;
  }
}

function scopePart({ isShare, source, hash }: MetadataScope): string {
  return isShare ? `share:${hash}` : `src:${source}`;
}

function normalizePath(path: string | undefined): string {
  const collapsed = `/${path || ""}`.replace(/\/+/g, "/");
  return collapsed.length > 1 ? collapsed.replace(/\/$/, "") : "/";
}

function keyOf(scope: MetadataScope, path: string, kind: string): string {
  return `${scopePart(scope)}|${normalizePath(path)}|${kind}`;
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
function stemOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return (dot > 0 ? name.slice(0, dot) : name).toLowerCase();
}

function trim(): void {
  let files = 0;
  let folderItems = 0;
  for (const { changes, size } of cache.values()) {
    if (changes) {
      folderItems += size;
    } else {
      files++;
    }
  }
  for (const [key, entry] of cache) {
    if (files <= MAX_FILE_ENTRIES && folderItems <= MAX_FOLDER_ITEMS) {
      break;
    }
    if (entry.changes ? folderItems > MAX_FOLDER_ITEMS : files > MAX_FILE_ENTRIES) {
      cache.delete(key);
      if (entry.changes) {
        folderItems -= entry.size;
      } else {
        files--;
      }
    }
  }
}

/** @param fresh true for a new fetch, the TTL then counts from when its result arrived */
function track(key: string, entry: Entry, fresh: boolean): void {
  const { promise } = entry;
  promise.then(
    (payload) => {
      if (cache.get(key) !== entry || entry.promise !== promise) {
        return;
      }
      // An empty folder answer is a valid answer, dropping it made every consumer
      // (listing, preview, next/previous) request the whole folder again. It counts as one item.
      if (!entry.changes && !payload) {
        cache.delete(key);
        return;
      }
      if (fresh) {
        entry.time = Date.now();
      }
      entry.size = payload instanceof Map ? Math.max(1, payload.size) : 1;
      trim();
    },
    () => {
      if (cache.get(key) === entry && entry.promise === promise) {
        cache.delete(key);
      }
    },
  );
}

/** Looking an entry up counts as using it, so the ones you keep returning to are removed last */
function lookup(key: string): Entry | null {
  const entry = cache.get(key);
  if (!entry) {
    return null;
  }
  cache.delete(key);
  if (Date.now() - entry.time > TTL_MS) {
    return null;
  }
  cache.set(key, entry);
  return entry;
}

function add(key: string, scope: MetadataScope, path: string, promise: Promise<unknown>, changes?: Map<string, Change>): Entry {
  const entry: Entry = { scope: scopePart(scope), path: normalizePath(path), promise, time: Date.now(), size: 0, changes };
  cache.set(key, entry);
  track(key, entry, true);
  return entry;
}

/**
 * Runs once per file and kind within the TTL, so the info prompt, the preview and repeated opens share one request. Failed or empty results are not kept.
 * Entries go when the file, or a sidecar with its name, changes.
 */
export function cachedItem<T>(scope: MetadataScope, path: string, kind: string, fetcher: () => Promise<T>): Promise<T> {
  checkUser();
  const key = keyOf(scope, path, kind);
  const hit = lookup(key);
  return (hit ?? add(key, scope, path, fetcher())).promise as Promise<T>;
}

// Applies the per-item changes recorded since the folder was cached, fetching only the changed items.
async function applyChanges(
  base: Promise<DirMetadataMap>,
  changes: Map<string, Change>,
  fetchAll: () => Promise<DirMetadataMap>,
  fetchOne: (name: string) => Promise<object | null>,
): Promise<DirMetadataMap> {
  const map = new Map(await base);
  const toRefresh = new Set<string>();
  for (const [name, change] of changes) {
    if (change === "removed") {
      map.delete(name);
    } else {
      toRefresh.add(name);
    }
  }
  const stems = new Set([...changes.keys()].map(stemOf));
  for (const name of map.keys()) {
    if (stems.has(stemOf(name))) {
      toRefresh.add(name);
    }
  }
  // returns the names that failed, others are applied
  const refresh = async (names: string[]): Promise<string[]> => {
    const failed: string[] = [];
    await Promise.all(
      names.map(async (name) => {
        try {
          const metadata = await fetchOne(name);
          if (metadata) {
            map.set(name, metadata);
          } else {
            map.delete(name);
          }
        } catch {
          failed.push(name);
        }
      }),
    );
    return failed;
  };
  const failed = await refresh([...toRefresh]);
  if (failed.length && (await refresh(failed)).length) {
    return fetchAll();
  }
  return map;
}

/**
 * Metadata of a folder's items by name, cached and shared for all the callers to avoid unnecessary
 * requests. Items changed (deleted, uploaded, moved) are patched with `fetchOne` instead of
 * refetching the whole folder with `fetchAll`.
 */
export function cachedDirectory(
  scope: MetadataScope,
  path: string,
  fetchAll: () => Promise<DirMetadataMap>,
  fetchOne: (name: string) => Promise<object | null>,
): Promise<DirMetadataMap> {
  checkUser();
  const key = keyOf(scope, path, DIR);
  const entry = lookup(key) ?? add(key, scope, path, fetchAll(), new Map());
  const { changes } = entry;
  if (changes?.size) {
    entry.changes = new Map();
    entry.promise = applyChanges(entry.promise as Promise<DirMetadataMap>, changes, fetchAll, fetchOne);
    track(key, entry, false);
  }
  return entry.promise as Promise<DirMetadataMap>;
}

function drop(match: (entry: Entry) => boolean): void {
  checkUser();
  for (const [key, entry] of cache) {
    if (match(entry)) {
      cache.delete(key);
    }
  }
}

/** an item and everything cached below it, for folders and files alike */
function dropSubtree(scope: MetadataScope, itemPath: string): void {
  const scopeKey = scopePart(scope);
  const root = normalizePath(itemPath);
  const prefix = root === "/" ? "/" : `${root}/`;
  drop((entry) => entry.scope === scopeKey && (entry.path === root || entry.path.startsWith(prefix)));
}

/** cached entries go when the file or a sidecar sharing its stem changes */
function dropFilesByStem(scope: MetadataScope, dir: string, stem: string): void {
  const scopeKey = scopePart(scope);
  const dirPath = normalizePath(dir);
  drop((entry) => {
    if (entry.changes || entry.scope !== scopeKey) {
      return false;
    }
    const parts = splitItemPath(entry.path);
    const entryStem = stemOf(parts.name);
    // "movie.en.srt" (stem "movie.en") changes the subtitles of "movie.mkv" (stem "movie")
    return parts.dir === dirPath && (entryStem === stem || stem.startsWith(`${entryStem}.`));
  });
}

function recordChange(scope: MetadataScope, itemPath: string, change: Change): void {
  const { dir, name, isDir } = splitItemPath(itemPath);
  if (!name) {
    return;
  }
  dropSubtree(scope, itemPath);
  if (change === "changed" && isDir) {
    return; // folders have no media metadata of their own
  }
  dropFilesByStem(scope, dir, stemOf(name));
  const key = keyOf(scope, dir, DIR);
  const changes = cache.get(key)?.changes;
  if (!changes) {
    return;
  }
  changes.set(name, change);
  if (changes.size > MAX_PENDING_CHANGES) {
    cache.delete(key);
  }
}

/** An item was deleted or moved away: drop only its entry */
export function removeDirMetadataItem(scope: MetadataScope, itemPath: string): void {
  recordChange(scope, itemPath, "removed");
}

/** An item was uploaded, copied or moved: refresh only its entry */
export function markDirMetadataItemChanged(scope: MetadataScope, itemPath: string): void {
  recordChange(scope, itemPath, "changed");
}

/** drops its folder's listing and everything cached below the item */
export function invalidateDirMetadataItem(scope: MetadataScope, itemPath: string): void {
  const { dir, name } = splitItemPath(itemPath);
  dropSubtree(scope, itemPath);
  if (name) {
    dropFilesByStem(scope, dir, stemOf(name));
  }
  cache.delete(keyOf(scope, dir, DIR));
}

/** Drops everything cached, so the next calls get fresh data */
export function invalidateDirMetadataCache(): void {
  cache.clear();
}
