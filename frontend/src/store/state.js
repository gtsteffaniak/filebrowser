import { reactive } from 'vue';
import { detectLocale } from "@/i18n";

export const state = reactive({
  disableOnlyOfficeExt: "",
  isSafari: /^((?!chrome|android).)*safari/i.test(navigator.userAgent),
  activeSettingsView: "",
  isMobile: window.innerWidth <= 800,
  isSearchActive: false,
  showSidebar: false,
  usages: {},
  editor: null,
  serverHasMultipleSources: false,
  sources: {
    current: "",
    count: 1,
    info: {},
  },
  user: {
    quickDownloadEnabled: false,
    gallarySize: 0,
    singleClick: false,
    stickySidebar: stickyStartup(),
    locale: detectLocale(), // Default to the locale from moment
    viewMode: 'normal', // Default to mosaic view
    showHidden: false, // Default to false, assuming this is a boolean
    scopes: [],
    perm: {},
    rules: [], // Default to an empty array
    permissions: {}, // Default to an empty object for permissions
    darkMode: true, // Default to false, assuming this is a boolean
    profile: { // Example of additional user properties
      username: '', // Default to an empty string
      email: '', // Default to an empty string
      avatarUrl: '' // Default to an empty string
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
  previewRaw: "",
  oldReq: {},
  clipboard: {
    key: "",
    items: [],
  },
  jwt: "",
  sharePassword: "",
  loading: [],
  reload: false,
  selected: [],
  multiple: false,
  upload: {
    uploads: {},
    queue: [],
    progress: [],
    sizes: [],
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
      disableUsedPercentage: false,
      name: "",
      files: "",
    },
  },
});

function stickyStartup() {
  const stickyStatus = localStorage.getItem("stickySidebar");
  return stickyStatus == "true"
}