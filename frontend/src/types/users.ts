import type { SidebarLink, SourceFilePermissions } from "./settings.ts";

export interface Permissions {
  share?: boolean;
  modify?: boolean;
  create?: boolean;
  delete?: boolean;
  download?: boolean;
  admin?: boolean;
  api?: boolean;
  archive?: boolean;
  realtime?: boolean;
}

export interface DisplayPreference {
  viewMode?: string;
  sorting?: {
    by: string;
    asc: boolean;
  };
}

export interface UserScope {
  name: string;
  scope: string;
  permissions?: SourceFilePermissions;
}

export interface UserObject {
  preview: {
    video: boolean;
    audio: boolean;
    image: boolean;
    models: boolean;
    popup: boolean;
    autoplayMedia?: boolean;
    office?: boolean;
    folder?: boolean;
    motionVideoPreview?: boolean;
    disableHideSidebar?: boolean;
  };
  loginType: string;
  username: string;
  quickDownloadEnabled: boolean;
  gallerySize: number;
  singleClick: boolean;
  stickySidebar: boolean;
  hideFilesInTree: boolean,
  deleteAfterArchive: boolean,
  deleteWithoutConfirming?: boolean;
  locale: string;
  viewMode: string;
  showHidden: boolean;
  disableSearchOptions?: boolean;
  scopes: UserScope[];
  permissions: Permissions;
  /** Theme preference: "auto" (follow system), "dark", "light" */
  darkMode: boolean | "auto" | "dark" | "light";
  disableSettings: boolean;
  disableUpdateNotifications?: boolean;
  showAdvancedProfile?: boolean;
  allowCustomShareURL?: boolean;
  debugOffice: boolean;
  preferEditorForMarkdown: boolean;
  preferEditorForHtml: boolean;
  showCopyPath?: boolean;
  showSelectMultiple?: boolean;
  showTypeColumn?: boolean;
  showCreationDateColumn?: boolean;
  hideFileExt?: string;
  newFileTemplate?: string[];
  themeColor?: string;
  sidebarLinks?: SidebarLink[];
  toolAccess?: Record<string, boolean>;
  effectiveToolAccess?: Record<string, boolean>;
  profile: {
    username: string;
    email: string;
    avatarUrl: string;
  };
  /** Optional properties that may be added dynamically */
  disableViewingExt?: string;
  disablePreviewExt?: string;
  disableOnlyOfficeExt?: string;
  displayNames?: string[];
  id?: number;
  password?: string;
  scope?: string;
  rules?: unknown[];
  lockPassword?: boolean;
  requirePasswordChange?: boolean;
  hideDotfiles?: boolean;
  sorting?: {
    by: string;
    asc: boolean;
  };
  dateFormat?: boolean;
  perm?: unknown;
  email?: string;
  avatarUrl?: string;
  fileLoading?: {
    maxConcurrentUpload?: number;
    uploadChunkSizeMb?: number;
    uploadStallTimeoutSeconds?: number;
    clearAll?: boolean;
    downloadChunkSizeMb?: number;
    maxConcurrent?: number;
    chunkSizeMb?: number;
  };
}
