import { vi } from 'vitest';

vi.mock('@/store', () => {
  return {
    state: {
      activeSettingsView: "",
      isMobile: false,
      showSidebar: false,
      usage: {
        used: "0 B",
        total: "0 B",
        usedPercentage: 0,
      },
      editor: null,
      user: {
        gallarySize: 0,
        stickySidebar: false,
        locale: "en",
        viewMode: "normal",
        hideDotfiles: false,
        perm: {},
        rules: [],
        permissions: {},
        darkMode: false,
        profile: {
          username: '',
          email: '',
          avatarUrl: '',
        },
      },
      req: {
        sorting: {
          by: 'name',
          asc: true,
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
    },
  };
});

vi.mock('@/utils/constants', () => {
  return {
    baseURL: "http://example.com",
  };
});