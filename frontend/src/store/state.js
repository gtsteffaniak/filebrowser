import { reactive } from 'vue';
import { detectLocale } from "@/i18n";

export const state = reactive({
  usage: {
    used: "0 B",
    total: "0 B",
    usedPercentage: 0
  },
  editor: null,
  user: {
    locale: detectLocale(), // Default to the locale from moment
    viewMode: 'mosaic', // Default to mosaic view
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
  progress: 0,
  loading: false,
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
  showShell: false,
  showConfirm: null,
  route: {},
});
