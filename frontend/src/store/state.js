import { reactive } from 'vue';
import { detectLocale } from "@/i18n";

export const state = reactive({
  disableEventThemes: eventTheme(),
  tooltip: {
    show: false,
    content: "",
    x: 0,
    y: 0,
    pointerEvents: false,
    width: null,
  },
  previousHistoryItem: {
    name: "",
    source: "",
    path: "",
  },
  contextMenuHasItems: false,
  deletedItem: false,
  showOverflowMenu: false,
  sessionId: "",
  isSafari: /^((?!chrome|android).)*safari/i.test(navigator.userAgent),
  activeSettingsView: "",
  isMobile: window.innerWidth <= 768,
  isSearchActive: false,
  showSidebar: false,
  displayPreferences: {},
  usages: {},
  editor: null,
  editorDirty: false,
  editorSaveHandler: null, // Function to save editor content
  realtimeActive: undefined,
  realtimeDownCount: 0,
  popupPreviewSourceInfo: null, // { source, path, size, url, modified } - set by Icon when hovering
  sources: {
    current: "",
    count: 1,
    hasSourceInfo: false,
    info: {},
  },
  user: {
    preview: {
      video: true,
      image: true,
      popup: true,
      highQuality: true,
    },
    loginType: "",
    username: "",
    quickDownloadEnabled: false,
    gallerySize: 0,
    singleClick: false,
    stickySidebar: stickyStartup(),
    locale: detectLocale(), // Default to the locale from moment
    viewMode: 'normal', // Default to mosaic view
    showHidden: false, // Default to false, assuming this is a boolean
    scopes: [],
    permissions: {}, // Default to an empty object for permissions
    darkMode: true, // Default to false, assuming this is a boolean
    disableSettings: false,
    debugOffice: false, // Debug mode for OnlyOffice integration
    profile: { // Example of additional user properties
      username: '', // Default to an empty string
      email: '', // Default to an empty string
      avatarUrl: '' // Default to an empty string
    },
    fileLoading: {
      maxConcurrentUpload: 3,
      uploadChunkSizeMb: 5,
      clearAll: false
    }
  },
  req: {
    sorting: {
      by: 'name', // Initial sorting field
      asc: true,  // Initial sorting order
    },
    items: [],
    numDirs: 0,
    numFiles: 0,
  },
  listing: {
    category: "folders",
    letter: "A",
    scrolling: false,
    scrollRatio: 0,
  },
  previewRaw: "",
  oldReq: {},
  clipboard: {
    key: "",
    items: [],
  },
  sharePassword: "",
  loading: [],
  reload: false,
  selected: [],
  lastSelectedIndex: null,
  multiple: false,
  upload: {
    uploads: {},
    queue: [],
    progress: [],
    sizes: [],
    isUploading: false,
  },
  prompts: [],
  show: null,
  showConfirm: null,
  route: {},
  settings: {
    signup: false,
    createUserDir: false,
    userHomeBasePath: "",
    rules: [],
    frontend: {
      disableExternal: false,
      name: "",
      files: "",
    },
  },
  navigation: {
    show: false,
    hoverNav: false,
    listing: null,
    currentIndex: -1,
    previousItem: null,
    nextItem: null,
    previousLink: "",
    nextLink: "",
    previousRaw: "",
    nextRaw: "",
    timeout: null,
    enabled: false,
    isTransitioning: false,
    transitionStartTime: null,
  },
  playbackQueue: {
    queue: [],
    currentIndex: -1,
    mode: 'single', // 'single', 'sequential', 'shuffle', 'loop-single', 'loop-all'
    isPlaying: false
  },
  shareInfo: {
    isShare: false,
    disableThumbnails: false,
    hash: "",
    token: "",
    subPath: "",
    passwordValid: false,
    enforceDarkLightMode: "",
    disableSidebar: false,
    isValid: true,
    shareType: "",
    title: "",
    description: "",
  },
  notificationHistory: loadNotificationHistory(), // Session-based notification history (persists across refreshes)
  sidebar: {
    width: getSidebarWidth(), // in em
    isResizing: false,
    minWidth: 17.5, // in em
    maxWidth: 37.5, // in em
  },
});

/**
 * Load notification history from sessionStorage
 * @returns {Array}
 */
function loadNotificationHistory() {
  try {
    const stored = sessionStorage.getItem('notificationHistory');
    return stored ? JSON.parse(stored) : [];
  } catch (error) {
    console.error('Failed to load notification history:', error);
    return [];
  }
}

/**
 * Load sidebar width from localStorage or use default size
 * @returns {number}
 */
function getSidebarWidth() {
  const savedWidth = localStorage.getItem("sidebarWidth");
  return savedWidth ? parseFloat(savedWidth) : 20; // 20em
}

function stickyStartup() {
  const stickyStatus = localStorage.getItem("stickySidebar");
  return stickyStatus == "true"
}

function eventTheme() {
  const disableEventThemes = localStorage.getItem("disableEventThemes");
  return disableEventThemes == "true"
}