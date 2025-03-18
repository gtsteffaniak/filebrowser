import { removePrefix } from "@/utils/url.js";
import { getFileExtension } from  "@/utils/files.js";
import { state } from "./state.js";
import { noAuth } from "@/utils/constants.js";
import { getTypeInfo } from "@/utils/mimetype";

export const getters = {
  previewType: () => getTypeInfo(state.req.type).simpleType,
  isCardView: () => (state.user.viewMode == "gallery" || state.user.viewMode == "normal" ) && getters.currentView() == "listingView" ,
  currentHash: () => state.route.hash.replace("#", ""),
  isMobile: () => state.isMobile,
  isLoading: () => Object.keys(state.loading).length > 0,
  isSettings: () => getters.currentView() === "settings",
  isShare: () => getters.currentView() === "share",
  isDarkMode: () => {
    if (state.user == null) {
      return true;
    }
    return state.user.darkMode === true;
  },
  isLoggedIn: () => {
    if (noAuth) {
      return true
    }
    if (state.user !== null && state.user?.username != "" && state.user?.username != "publicUser") {
      return true;
    }
    return false
  },
  isAdmin: () => state.user.perm?.admin == true,
  isFiles: () => state.route.name === "Files",
  isListing: () => getters.isFiles() && state.req.type === "directory",
  selectedCount: () => Array.isArray(state.selected) ? state.selected.length : 0,
  getFirstSelected: () => state.req.items[state.selected[0]],
  isSingleFileSelected: () => getters.selectedCount() === 1 && getters.getFirstSelected()?.type != "directory",
  selectedDownloadUrl() {
    if (state.isSearchActive) {
      return state.selected[0].url;
    }
    let selectedItem = state.selected[0]
    return state.req.items[selectedItem].url;
  },
  reqNumDirs: () => {
    let dirCount = 0;
    state.req.items.forEach((item) => {
      // Check if the item is a directory
      if (item.type == "directory") {
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
      if (item.type != "directory") {
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
      if (item.type == "directory") {
        dirs.push(item);
      } else {
        item.Path = state.req.Path;
        files.push(item);
      }
    });
    return { dirs, files };
  },
  isSidebarVisible: () => {
    const currentView = getters.currentView();
    let visible = (state.showSidebar || getters.isStickySidebar()) && state.user.username != "publicUser"
    if (currentView == "settings") {
      visible = !getters.isMobile();
    }
    if (currentView == "share") {
      visible = false
    }
    if (typeof getters.currentPromptName() === "string" && !getters.isStickySidebar()) {
      visible = false;
    }
    if (currentView == "preview") {
      const contentType = getters.previewType()
      if (
        contentType == "audio" ||
        contentType == "video" ||
        contentType == "image" ||
        contentType == "pdf"
      ) {
        visible = false
      }
    }
    if (
      currentView == "editor" ||
      currentView == "onlyOfficeEditor"
      ) {
      visible = false;
    }
    return visible
  },
  isStickySidebar: () => {
    let sticky = state.user?.stickySidebar;
    const currentView = getters.currentView();
    if (currentView == "settings") {
      sticky = true
    }
    if (currentView == null && !getters.isLoading() && getters.isShare() ) {
      sticky = true
    }
    if (getters.isMobile()) {
      sticky = false
    }
    return sticky
  },
  showOverlay: () => {
    const hasPrompt = getters.currentPrompt() !== null && getters.currentPromptName() !== "more"
    const showForSidebar = getters.isSidebarVisible() && !getters.isStickySidebar()
    return hasPrompt || showForSidebar || state.isSearchActive;
  },
  showBreadCrumbs: () => {
    return getters.currentView() == "listingView" ;
  },
  routePath: (trimModifier="") => {
    return removePrefix(state.route.path,trimModifier)
  },
  sharePathBase: () => {
    let urlPath = getters.routePath("share");
    // Step 1: Split the path by '/'
    let parts = urlPath.split("/");
    // Step 2: Assign hash to the second part (index 2) and join the rest for subPath
    return "/share/"+ parts[1];
  },
  currentView: () => {
    const pathname = getters.routePath()
    if (pathname.startsWith(`/settings`)) {
      return "settings"
    } else if (pathname.startsWith(`/share`)) {
      return "share"
    } else if (pathname.startsWith(`/files`)) {
      if (state.req.type !== undefined) {
        if (state.req.type == "directory") {
          return "listingView";
        } else if (getters.onlyOfficeEnabled(state.req.name)) {
          return "onlyOfficeEditor";
        } else if ("content" in state.req && state.req.type == "text/markdown" && window.location.hash != "#edit") {
          return "markdownViewer";
        } else if ("content" in state.req) {
          return "editor";
        } else {
          return "preview";
        }
      }
    }
    return null
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
      let isDir = upload.file.type == "directory";
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
  onlyOfficeEnabled: (filename) => {
    if (!state.req?.onlyOfficeId) {
      return false
    }
    const ext = getFileExtension(filename);
    if (state.user.disableOnlyOfficeExt) {
      const disabledList = state.user.disableOnlyOfficeExt.split(" ");
      for (const e of disabledList) {
        if (e.trim().toLowerCase() === ext.toLowerCase()) {
          return false;
        }
      }
    }
    return true;
  },
};
