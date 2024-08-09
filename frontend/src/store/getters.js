import { state } from "./state.js";

export const getters = {
  isMobile: () => state.isMobile,
  isDarkMode: () => {
    if (state.user == null) {
      return true;
    }
    return state.user.darkMode === true;
  },
  isLoggedIn: () => {
    return state.user !== null && state.user?.username != undefined && state.user?.username != "publicUser";
  },
  isAdmin: () => state.user.perm?.admin == true,
  isFiles: () => state.route.name === "Files",
  isListing: () => getters.isFiles() && state.req.isDir,
  selectedCount: () => Array.isArray(state.selected) ? state.selected.length : 0,
  isSingleFileSelected: () => getters.selectedCount() === 1 && !state.req.items[state.selected[0]]?.isDir,
  selectedDownloadUrl() {
    let selectedItem = state.selected[0]
    return state.req.items[selectedItem].url;
  },
  reqNumDirs: () => {
    let dirCount = 0;
    state.req.items.forEach((item) => {
      // Check if the item is a directory
      if (item.isDir) {
        // If hideDotfiles is enabled and the item is a dotfile, skip it
        if (state.user.hideDotfiles && item.name.startsWith(".")) {
          return;
        }
        // Otherwise, count this directory
        dirCount++;
      }
    });
    // Return the directory count
    return dirCount;
  },
  reqNumFiles: () => {
    let fileCount = 0;
    state.req.items.forEach((item) => {
      // Check if the item is a directory
      if (!item.isDir) {
        // If hideDotfiles is enabled and the item is a dotfile, skip it
        if (state.user.hideDotfiles && item.name.startsWith(".")) {
          return;
        }
        // Otherwise, count this directory
        fileCount++;
      }
    });
    // Return the directory count
    return fileCount;
  },
  reqItems: () => {
    if (state.user == null) {
      return {};
    }
    const dirs = [];
    const files = [];

    state.req.items.forEach((item) => {
      if (state.user.hideDotfiles && item.name.startsWith(".")) {
        return;
      }
      if (item.isDir) {
        dirs.push(item);
      } else {
        item.Path = state.req.Path;
        files.push(item);
      }
    });
    return { dirs, files };
  },
  isSidebarVisible: () => {
    if (!getters.isLoggedIn()) {
      return false;
    }
    if (typeof getters.currentPromptName() === "string" && !getters.isStickySidebar()) {
      return false;
    }
    if (getters.currentView() !== "listingView") {
      return false;
    }
    return state.showSidebar || getters.isStickySidebar();
  },
  isStickySidebar: () => {
    if (getters.isMobile()) {
      return false
    }
    if (!getters.isLoggedIn()) {
      return true
    }
    return state.user?.stickySidebar
  },
  showOverlay: () => {
    if (!getters.isLoggedIn()) {
      return false
    }
    const hasPrompt = getters.currentPrompt() !== null && getters.currentPromptName() !== "more";
    const shouldOverlaySidebar = getters.isSidebarVisible() && !getters.isStickySidebar()
    return hasPrompt || shouldOverlaySidebar;
  },
  getRoutePath: () => {
    return state.route.path.endsWith("/")
    ? state.route.path
    : state.route.path + "/";
  },
  currentView: () => {
    let returnVal = null;
    if (state.req.type !== undefined) {
      if (state.req.isDir) {
        returnVal = "listingView";
      } else if ("content" in state.req) {
        returnVal = "editor";
      } else {
        returnVal = "preview";
      }
    }
    return returnVal;
  },
  progress: () => {
    // Check if state.upload is defined and valid
    if (!state.upload || !Array.isArray(state.upload.progress) || !Array.isArray(state.upload.sizes)) {
      return 0;
    }

    // Handle cases where progress or sizes arrays might be empty
    if (state.upload.progress.length === 0 || state.upload.sizes.length === 0) {
      return 0;
    }

    // Calculate totalSize
    let totalSize = state.upload.sizes.reduce((a, b) => a + b, 0);

    // Calculate sum of progress
    let sum = state.upload.progress.reduce((acc, val) => acc + val, 0);

    // Return progress as a percentage
    return Math.ceil((sum / totalSize) * 100);
  },

  filesInUploadCount: () => {
    const uploadsCount = state.upload.length
    const queueCount = state.queue.length
    return uploadsCount + queueCount;
  },

  currentPrompt: () => {
    // Ensure state.prompts is an array
    if (!Array.isArray(state.prompts)) {
      return null;
    }
    if (state.prompts.length === 0) {
      return null;
    }
    return state.prompts[state.prompts.length - 1]
  },

  currentPromptName: () => {
    // Ensure state.prompts is an array
    if (!Array.isArray(state.prompts) || state.prompts.length === 0) {
      return null;
    }
    // Check if the name property is a string
    const lastPrompt = state.prompts[state.prompts.length - 1];
    if (typeof lastPrompt?.name !== "string") {
      return null;
    }
    return lastPrompt.name;
  },

  filesInUpload: () => {
    // Ensure state.upload.uploads is an object and state.upload.sizes is an array
    if (typeof state.upload.uploads !== 'object' || !Array.isArray(state.upload.sizes)) {
      return [];
    }

    let files = [];

    for (let index in state.upload.uploads) {
      let upload = state.upload.uploads[index];
      let id = upload.id;
      let type = upload.type;
      let name = upload.file.name;
      let size = state.upload.sizes[id] || 0; // Default to 0 if size is undefined
      let isDir = upload.file.isDir;
      let progress = isDir
        ? 100
        : Math.ceil((state.upload.progress[id] || 0 / size) * 100); // Default to 0 if progress is undefined

      files.push({
        id,
        name,
        progress,
        type,
        isDir,
      });
    }

    return files.sort((a, b) => a.progress - b.progress);
  },
};
