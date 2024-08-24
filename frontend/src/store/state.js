import { reactive } from 'vue';
import { detectLocale } from "@/i18n";

export const state = reactive({
  activeSettingsView: "",
  isMobile: window.innerWidth <= 800,
  showSidebar: false,
  usage: {
    used: "0 B",
    total: "0 B",
    usedPercentage: 0
  },
  editor: null,
  user: {
    gallarySize: 0,
    stickySidebar: stickyStartup(),
    locale: detectLocale(), // Default to the locale from moment
    viewMode: 'normal', // Default to mosaic view
    hideDotfiles: false, // Default to false, assuming this is a boolean
    perm: {},
    rules: [], // Default to an empty array
    permissions: {}, // Default to an empty object for permissions
    darkMode: false, // Default to false, assuming this is a boolean
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
  oldReq: {},
  clipboard: {
    key: "",
    items: [],
  },
  jwt: "",
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