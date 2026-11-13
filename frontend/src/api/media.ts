import { notify } from "@/notify/index.ts";
import { getters, state } from "@/store/index.ts";
import type { MediaMetadata, SubtitleTrack } from "@/store/types.ts";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { adjustedData, fetchURL, type ListingResponse } from "./utils.ts";
import { cachedDirectory, cachedItem, type DirMetadataMap, type MetadataScope } from "@/utils/cache/metadataCache.ts";

/** GET /api/media/metadata: a directory with the metadata of its items, or a single file */
export type MediaResource = ListingResponse & {
  metadata?: MediaMetadata;
  subtitles?: SubtitleTrack[];
  hasPreview?: boolean;
};

export interface Lyrics {
  lyrics: string;
  format: string;
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : "";
}

// Inside a share the requests go to the public API, otherwise to the source.
// Decided here so callers don't branch, and it is the scope of what gets cached.
function mediaScope(source?: string): MetadataScope {
  const hash = getters.shareHash();
  return hash ? { isShare: true, hash } : { source };
}

// GET /api/media/{endpoint} or /public/api/media/{endpoint}
function fetchMedia(endpoint: string, scope: MetadataScope, params: Record<string, string>): Promise<Response> {
  if (scope.isShare) {
    const password = localStorage.getItem(`sharepass:${scope.hash}`) || "";
    return fetchURL(getPublicApiPath(`media/${endpoint}`, { hash: scope.hash, ...params }), {
      headers: { "X-SHARE-PASSWORD": encodeURIComponent(password) },
    });
  }
  return fetchURL(getApiPath(`media/${endpoint}`, { source: scope.source, ...params }));
}

export function getSubtitleContent(source: string, path: string, subtitleName: string, embedded = false): Promise<string> {
  const scope = mediaScope(source);
  return cachedItem(scope, path, `subtitle:${subtitleName}:${embedded}`, async () => {
    try {
      const res = await fetchMedia("subtitles", scope, {
        path,
        name: subtitleName,
        ...(embedded && { embedded: "true" }),
      });
      return await res.text();
    } catch (err) {
      notify.showError(errorMessage(err) || `Error fetching subtitle ${subtitleName}`);
      throw err;
    }
  });
}

export function getLyrics(source: string, path: string): Promise<Lyrics> {
  const scope = mediaScope(source);
  return cachedItem(scope, path, "lyrics", async () => {
    const res = await fetchMedia("lyrics", scope, { path });
    const data = await res.json();
    return { lyrics: data.lyrics || "", format: data.format || "" };
  });
}

// albumArt adds the embedded cover of audio files.
async function fetchMetadata(scope: MetadataScope, path: string, albumArt = false): Promise<MediaResource> {
  const res = await fetchMedia("metadata", scope, { path, ...(albumArt && { albumArt: "true" }) });
  return adjustedData(await res.json());
}

/** Metadata of a single file */
export function getItemMediaMetadata(
  path: string,
  { source, albumArt = false }: { source?: string; albumArt?: boolean } = {},
): Promise<MediaResource> {
  const scope = mediaScope(source);
  const fetcher = () => fetchMetadata(scope, path, albumArt);
  return albumArt ? fetcher() : cachedItem(scope, path, "metadata", fetcher);
}

/**
 * Metadata of the items of a directory by name, cached by dir and shared for all callers.
 * Items changed are patched individually instead of refetching the whole dir again.
 */
export function getDirectoryMetadataMap(path: string, { source }: { source?: string } = {}): Promise<DirMetadataMap> {
  const scope = mediaScope(source);
  const base = !path || path === "/" ? "" : path.replace(/\/+$/, "");
  return cachedDirectory(
    scope,
    path,
    async () => {
      const payload = await fetchMetadata(scope, path);
      const withMetadata = (payload.items || []).filter((i) => i.metadata);
      return new Map(withMetadata.map((i) => [i.name, i.metadata as object]));
    },
    async (name) => {
      const payload = await fetchMetadata(scope, `${base}/${name}`);
      return payload.type !== "directory" && payload.metadata ? payload.metadata : null;
    },
  );
}

// GET /api/media/stream or /public/api/media/stream — audio/video bytes via viewToken (range-based, not download-metered).
export function getStreamURL(source: string, path: string, viewToken: string): string {
  const scope = mediaScope(source);
  if (!scope.isShare && !source) {
    throw new Error("no source provided");
  }
  if (!viewToken) {
    throw new Error("view token required");
  }
  const params = { file: path, viewToken: viewToken, sessionId: state.sessionId };
  const apiPath = scope.isShare
    ? getPublicApiPath("media/stream", { hash: scope.hash, ...params })
    : getApiPath("media/stream", { source, ...params });
  return window.origin + apiPath;
}
