import type { MediaMetadata, SubtitleTrack } from "./media.ts";

export interface FileListItem {
  name: string;
  path: string;
  size?: number;
  type?: string;
  source?: string;
  modified?: string;
  created?: string;
  hasPreview?: boolean;
  viewToken?: string;
  isShared?: boolean;
  pinned?: boolean;
  hidden?: boolean;
  metadata?: MediaMetadata;
}

/** List items have their position in `req.items` (by mutations.replaceRequest) */
export interface fileListItemIndex extends FileListItem {
  index: number;
}

export interface InfoItem extends FileListItem {
  subtitles?: SubtitleTrack[];
  token?: string;
  hash?: string;
  onlyOfficeId?: string;
}

export interface ReqObject {
  // Base properties always present
  sorting: {
    by: string;
    asc: boolean;
  };
  items: FileListItem[];
  numDirs: number;
  numFiles: number;

  // File-specific properties (added dynamically)
  name?: string;
  path?: string;
  size?: number;
  type?: string;
  source?: string;
  content?: string;
  modified?: string;
  created?: string;
  hasPreview?: boolean;
  subtitles?: SubtitleTrack[];
  viewToken?: string;
  parentDirItems?: FileListItem[];
  onlyOfficeId?: string;
  hasUpdate?: boolean;
  isDir?: boolean;
  metadata?: MediaMetadata;
  listing?: FileListItem[];
}

/** A listing item as the API sends it: only the name is guaranteed. */
export type RawListingItem = Partial<FileListItem> & { name: string };

export interface ListingResponse {
  type?: string;
  source?: string;
  path?: string;
  pinnedItems?: string[];
  folders?: RawListingItem[];
  files?: RawListingItem[];
  items?: RawListingItem[];
}
