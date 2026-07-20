/**
 * Maps flat runtime user objects ↔ nested user-defaults / profile sections (matches backend ProfileFromUser).
 */

function boolPtr(val, defaultValue = true) {
  if (val === undefined || val === null) {
    return defaultValue;
  }
  return !!val;
}

export function emptyProfileSections() {
  return {
    sidebar: {},
    listing: {},
    preview: {},
    fileViewer: {},
    search: {},
    ui: {},
    account: { permissions: {} },
    fileLoading: {},
  };
}

export function sectionsFromFlatUser(user) {
  const u = user || {};
  const preview = u.preview || {};
  return {
    sidebar: {
      disableQuickToggles: !!u.disableQuickToggles,
      hideFileActions: !!u.hideSidebarFileActions,
      disableHideOnPreview: !!preview.disableHideSidebar,
      sticky: !!u.stickySidebar,
      hideFiles: !!u.hideFilesInTree,
      showTools: u.showToolsInSidebar === undefined || u.showToolsInSidebar === null
        ? true
        : !!u.showToolsInSidebar,
    },
    listing: {
      deleteWithoutConfirming: !!u.deleteWithoutConfirming,
      dateFormat: !!u.dateFormat,
      showHidden: !!u.showHidden,
      quickDownload: !!u.quickDownload,
      showSelectMultiple: !!u.showSelectMultiple,
      singleClick: !!u.singleClick,
      hideFileExt: u.hideFileExt || "",
      showCopyPath: !!u.showCopyPath,
      deleteAfterArchive: !!u.deleteAfterArchive,
      viewMode: u.viewMode,
      gallerySize: u.gallerySize,
    },
    preview: {
      image: boolPtr(preview.image),
      video: boolPtr(preview.video),
      audio: boolPtr(preview.audio),
      office: boolPtr(preview.office),
      folder: boolPtr(preview.folder),
      models: boolPtr(preview.models),
      popup: boolPtr(preview.popup),
      motionVideoPreview: boolPtr(preview.motionVideoPreview),
      disablePreviewExt: u.disablePreviewExt || "",
    },
    fileViewer: {
      defaultMediaPlayer: !!preview.defaultMediaPlayer,
      autoplayMedia: boolPtr(preview.autoplayMedia),
      editorQuickSave: !!u.editorQuickSave,
      preferEditorForMarkdown: !!u.preferEditorForMarkdown,
      debugOffice: !!u.debugOffice,
      disableViewingExt: u.disableViewingExt || "",
      disableOnlyOfficeExt: u.disableOnlyOfficeExt || "",
    },
    search: {
      disableOptions: !!u.disableSearchOptions,
    },
    ui: {
      darkMode: boolPtr(u.darkMode),
      themeColor: u.themeColor || "",
      customTheme: u.customTheme || "",
      locale: u.locale || "",
    },
    account: {
      lockPassword: !!u.lockPassword,
      disableSettings: !!u.disableSettings,
      disableUpdateNotifications: !!u.disableUpdateNotifications,
      loginMethod: u.loginMethod || "",
      permissions: { ...(u.permissions || {}) },
    },
    fileLoading: { ...(u.fileLoading || {}) },
  };
}

export function applySectionsToFlatUser(user, sections) {
  if (!user || !sections) {
    return;
  }
  if (!user.preview) {
    user.preview = {};
  }
  const s = sections;
  const sidebar = s.sidebar || {};
  const listing = s.listing || {};
  const preview = s.preview || {};
  const fileViewer = s.fileViewer || {};
  const search = s.search || {};
  const ui = s.ui || {};
  const account = s.account || {};

  user.disableQuickToggles = !!sidebar.disableQuickToggles;
  user.hideSidebarFileActions = !!sidebar.hideFileActions;
  user.preview.disableHideSidebar = !!sidebar.disableHideOnPreview;
  user.stickySidebar = !!sidebar.sticky;
  user.hideFilesInTree = !!sidebar.hideFiles;
  user.showToolsInSidebar = sidebar.showTools === undefined || sidebar.showTools === null
    ? true
    : !!sidebar.showTools;

  user.deleteWithoutConfirming = !!listing.deleteWithoutConfirming;
  user.dateFormat = !!listing.dateFormat;
  user.showHidden = !!listing.showHidden;
  user.quickDownload = !!listing.quickDownload;
  user.showSelectMultiple = !!listing.showSelectMultiple;
  user.singleClick = !!listing.singleClick;
  user.hideFileExt = listing.hideFileExt || "";
  user.showCopyPath = !!listing.showCopyPath;
  user.deleteAfterArchive = !!listing.deleteAfterArchive;
  if (listing.viewMode !== undefined) {
    user.viewMode = listing.viewMode;
  }
  if (listing.gallerySize !== undefined) {
    user.gallerySize = listing.gallerySize;
  }

  user.preview.image = boolPtr(preview.image);
  user.preview.video = boolPtr(preview.video);
  user.preview.audio = boolPtr(preview.audio);
  user.preview.office = boolPtr(preview.office);
  user.preview.folder = boolPtr(preview.folder);
  user.preview.models = boolPtr(preview.models);
  user.preview.popup = boolPtr(preview.popup);
  user.preview.motionVideoPreview = boolPtr(preview.motionVideoPreview);
  user.disablePreviewExt = preview.disablePreviewExt || "";

  user.preview.defaultMediaPlayer = !!fileViewer.defaultMediaPlayer;
  user.preview.autoplayMedia = boolPtr(fileViewer.autoplayMedia);
  user.editorQuickSave = !!fileViewer.editorQuickSave;
  user.preferEditorForMarkdown = !!fileViewer.preferEditorForMarkdown;
  user.debugOffice = !!fileViewer.debugOffice;
  user.disableViewingExt = fileViewer.disableViewingExt || "";
  user.disableOnlyOfficeExt = fileViewer.disableOnlyOfficeExt || "";

  user.disableSearchOptions = !!search.disableOptions;

  user.darkMode = boolPtr(ui.darkMode);
  user.themeColor = ui.themeColor || "";
  user.customTheme = ui.customTheme || "";
  if (ui.locale !== undefined) {
    user.locale = ui.locale;
  }

  if (account.lockPassword !== undefined) {
    user.lockPassword = !!account.lockPassword;
  }
  if (account.disableSettings !== undefined) {
    user.disableSettings = !!account.disableSettings;
  }
  if (account.disableUpdateNotifications !== undefined) {
    user.disableUpdateNotifications = !!account.disableUpdateNotifications;
  }
  if (account.loginMethod) {
    user.loginMethod = account.loginMethod;
  }
  if (account.permissions) {
    user.permissions = { ...(user.permissions || {}), ...account.permissions };
  }
  if (s.fileLoading) {
    user.fileLoading = { ...(user.fileLoading || {}), ...s.fileLoading };
  }
}
