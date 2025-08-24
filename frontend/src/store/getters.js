import { removePrefix, buildItemUrl, removeLeadingSlash } from '@/utils/url.js'
import { getFileExtension } from '@/utils/files.js'
import { state, mutations } from '@/store'
import { noAuth, shareInfo } from '@/utils/constants.js'
import { getTypeInfo } from '@/utils/mimetype'
import { fromNow } from '@/utils/moment'
import * as i18n from '@/i18n'

export const getters = {
  getTime: timestamp => {
    if (state.user.dateFormat) {
      // Truncate the fractional seconds to 3 digits (milliseconds)
      const sanitizedString = timestamp.replace(/\.\d+/, match =>
        match.slice(0, 4)
      )
      // Parse the sanitized string into a Date object
      const date = new Date(sanitizedString)
      return date.toLocaleString()
    }
    return fromNow(timestamp, state.user.locale)
  },
  isScrollable: () => {
    const cv = getters.currentView()
    if (
      cv == 'preview' ||
      cv == 'onlyOfficeEditor' ||
      cv == 'epubViewer' ||
      cv == 'docViewer' ||
      cv == 'editor'
    ) {
      return false
    }
    return true
  },
  previewType: () => getTypeInfo(state.req.type).simpleType,
  isCardView: () =>
    (state.user.viewMode == 'gallery' || state.user.viewMode == 'normal') &&
    getters.currentView() == 'listingView',
  currentHash: () => shareInfo.hash,
  isMobile: () => state.isMobile,
  isLoading: () => Object.keys(state.loading).length > 0,
  isSettings: () => getters.currentView() === 'settings',
  isShare: () => {
    return shareInfo.isShare
  },
  isDarkMode: () => {
    if (state.user == null) {
      return true
    }
    return state.user.darkMode === true
  },
  isLoggedIn: () => {
    if (state.user == null) {
      return false
    }
    if (state.user.locale == undefined || state.user.locale == null) {
      let savedLocale = localStorage.getItem('userLocale')
      if (!savedLocale) {
        savedLocale = i18n.detectLocale()
      }
      mutations.updateCurrentUser({ locale: savedLocale })
    }
    if (noAuth) {
      return true
    }
    if (
      state.user !== null &&
      state.user?.username != '' &&
      state.user?.username != 'anonymous'
    ) {
      return true
    }
    return false
  },
  isAdmin: () => state.user.permissions?.admin == true,
  isFiles: () => state.route.name === 'Files',
  isListing: () => getters.isFiles() && state.req.type === 'directory',
  selectedCount: () =>
    Array.isArray(state.selected) ? state.selected.length : 0,
  getFirstSelected: () => typeof(state.selected[0]) == 'number' ? state.req.items[state.selected[0]] : state.selected[0],
  isSingleFileSelected: () =>
    getters.selectedCount() === 1 &&
    getters.getFirstSelected()?.type != 'directory',
  selectedDownloadUrl () {
    if (state.isSearchActive) {
      return buildItemUrl(state.selected[0].source, state.selected[0].path)
    }
    return buildItemUrl(state.req.items[state.selected[0]].source, state.req.items[state.selected[0]].path)
  },
  reqNumDirs: () => {
    let dirCount = 0
    if (!state.req.items) {
      return 0
    }
    state.req.items.forEach(item => {
      // Check if the item is a directory
      if (item.type == 'directory') {
        // Otherwise, count this directory
        dirCount++
      }
    })
    // Return the directory count
    return dirCount
  },
  reqNumFiles: () => {
    let fileCount = 0
    if (!state.req.items) {
      return 0
    }
    state.req.items.forEach(item => {
      // Check if the item is a directory
      if (item.type != 'directory') {
        // Otherwise, count this directory
        fileCount++
      }
    })
    // Return the directory count
    return fileCount
  },
  reqItems: () => {
    if (state.user == null) {
      return {}
    }
    const dirs = []
    const files = []
    if (!state.req.items) {
      return { dirs, files }
    }
    state.req.items.forEach(item => {
      if (item.type == 'directory') {
        dirs.push(item)
      } else {
        item.Path = state.req.Path
        files.push(item)
      }
    })
    return { dirs, files }
  },
  isSidebarVisible: () => {
    if (shareInfo.disableSidebar) {
      return false
    }
    const cv = getters.currentView()
    if (cv == 'onlyOfficeEditor') {
      return false
    }
    const previewViews = [
      'preview',
      'markdownViewer',
      'epubViewer',
      'docViewer',
      'onlyOfficeEditor',
      'editor'
    ]

    let visible = (state.showSidebar || getters.isStickySidebar())
    if (getters.currentPromptName() && !getters.isStickySidebar()) {
      visible = false
    }
    if (previewViews.includes(cv) && !state.user.preview?.disableHideSidebar) {
      visible = false
    }
    return visible
  },
  isStickySidebar: () => {
    let sticky = state.user?.stickySidebar
    const currentView = getters.currentView()
    if (currentView == 'settings') {
      sticky = true
    }
    if (currentView == '' && !getters.isLoading()) {
      sticky = true
    }
    if (getters.isMobile()) {
      sticky = false
    }
    return sticky
  },
  showOverlay: () => {
    const hasPrompt =
      getters.currentPrompt() !== null && getters.currentPromptName() !== 'more'
    const showForSidebar =
      getters.isSidebarVisible() && !getters.isStickySidebar()
    return hasPrompt || showForSidebar || state.isSearchActive
  },
  showBreadCrumbs: () => {
    return getters.currentView() == 'listingView'
  },
  routePath: (trimModifier = '') => {
    return removePrefix(state.route.path, trimModifier)
  },
  shareHash: () => {
    return shareInfo.hash
  },
  sharePathBase: () => {
    return '/public/share/' + shareInfo.hash + '/'
  },
  getSharePath: (subPath = "") => {
    let urlPath = getters.routePath('public/share')
    let path =  "/" + removeLeadingSlash(urlPath.split(shareInfo.hash)[1])
    if (subPath != "") {
      path += "/" + removeLeadingSlash(subPath)
    }
    return path
  },
  currentView: () => {
    let listingView = ''
    const pathname = getters.routePath()
    if (pathname.startsWith(`/settings`)) {
      listingView = 'settings'
    } else {
      if (state.req.type !== undefined) {
        const ext = "." + state.req.name.split(".").pop().toLowerCase(); // Ensure lowercase and dot
        if (state.user.disableViewingExt?.includes(ext)) {
          return 'preview'
        }
        if (state.req.type == 'directory') {
          listingView = 'listingView'
        } else if (state.req.onlyOfficeId && !getters.officeViewingDisabled(state.req.name)) {
          listingView = 'onlyOfficeEditor'
        } else if (
          'content' in state.req &&
          state.req.type == 'text/markdown' &&
          window.location.hash != '#edit'
        ) {
          listingView = 'markdownViewer'
        } else if ('content' in state.req) {
          listingView = 'editor'
        } else if (state.req.type.startsWith('application/epub')) {
          listingView = 'epubViewer'
        } else if (
          state.req.type.startsWith(
            'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
          )
        ) {
          listingView = 'docViewer'
        } else {
          listingView = 'preview'
        }
      } else {
        listingView = 'listingView'
      }
    }
    return listingView
  },
  progress: () => {
    // Check if state.upload is defined and valid
    if (
      !state.upload ||
      !Array.isArray(state.upload.progress) ||
      !Array.isArray(state.upload.sizes)
    ) {
      return 0
    }

    // Handle cases where progress or sizes arrays might be empty
    if (state.upload.progress.length === 0 || state.upload.sizes.length === 0) {
      return 0
    }

    // Calculate totalSize
    let totalSize = state.upload.sizes.reduce((a, b) => a + b, 0)

    // Calculate sum of progress
    let sum = state.upload.progress.reduce((acc, val) => acc + val, 0)

    // Return progress as a percentage
    return Math.ceil((sum / totalSize) * 100)
  },

  filesInUploadCount: () => {
    const uploadsCount = state.upload.length
    const queueCount = state.queue.length
    return uploadsCount + queueCount
  },

  currentPrompt: () => {
    // Ensure state.prompts is an array
    if (!Array.isArray(state.prompts)) {
      return null
    }
    if (state.prompts.length === 0) {
      return null
    }
    return state.prompts[state.prompts.length - 1]
  },

  currentPromptName: () => {
    // Ensure state.prompts is an array
    if (!Array.isArray(state.prompts) || state.prompts.length === 0) {
      return ""
    }
    // Check if the name property is a string
    const lastPrompt = state.prompts[state.prompts.length - 1]
    if (typeof lastPrompt?.name !== 'string') {
      return ""
    }
    return lastPrompt.name
  },

  filesInUpload: () => {
    // Ensure state.upload.uploads is an object and state.upload.sizes is an array
    if (
      typeof state.upload.uploads !== 'object' ||
      !Array.isArray(state.upload.sizes)
    ) {
      return []
    }

    let files = []

    for (let index in state.upload.uploads) {
      let upload = state.upload.uploads[index]
      let id = upload.id
      let type = upload.type
      let name = upload.file.name
      let size = state.upload.sizes[id] || 0 // Default to 0 if size is undefined
      let isDir = upload.file.type == 'directory'
      let progress = isDir
        ? 100
        : Math.ceil((state.upload.progress[id] || 0 / size) * 100) // Default to 0 if progress is undefined

      files.push({
        id,
        name,
        progress,
        type,
        isDir
      })
    }

    return files.sort((a, b) => a.progress - b.progress)
  },
  fileViewingDisabled: filename => {
    const ext = ' ' + getFileExtension(filename)
    if (state.user.disableViewingExt) {
      const disabledExts = ' ' + state.user.disableViewingExt.toLowerCase()
      if (disabledExts.includes(ext.toLowerCase())) {
        return true
      }
    }
    return false
  },
  officeViewingDisabled: filename => {
    if (shareInfo.isShare) {
      return true
    }
    const ext = ' ' + getFileExtension(filename)
    if (state.user.disableOfficePreviewExt) {
      const disabledExts = ' ' + state.user.disableOfficePreviewExt.toLowerCase()
      if (disabledExts.includes(ext.toLowerCase())) {
        return true
      }
    }
    return false
  },
  anonymous: () => {
    return {
      id: 0,
      username: "anonymous",
      locale: i18n.detectLocale(),
      sorting: {
        by: "name",
        asc: true
      },
      viewMode: "normal",
      singleClick: true,
      quickDownload: false,
      gallerySize: 5,
      permissions: {
        share: false,
        modify: false,
        api: false,
        admin: false,
        realtime: false
      },
      preview: {
        video: true,
        image: true,
        popup: true,
        highQuality: false
      },
      disableSettings: true,
      disableQuickToggles: false,
      disableSearchOptions: false,
      deleteWithoutConfirming: false,
      stickySidebar: true,
      darkMode: true,
      dateFormat: false,
      disableViewingExt: "",
      disableOfficePreviewExt: "",
      disablePreviewExt: "",
      fileLoading: {
        maxConcurrent: 1,
        chunkSizeMb: 15,
      }
    }
  },
  multibuttonState: () => {
    const cv = getters.currentView()
    const isSidebarVisible = getters.isSidebarVisible()
    if (isSidebarVisible) {
      if (cv == "settings") {
        if (state.isMobile) {
          return "back";
        }
        return "close";
      }
      if (cv == "listingView") {
        if (state.user.stickySidebar) {
          return "menu";
        }
        return "back";
      }
      return "close";
    }
    if (cv == "settings") {
      if (state.isMobile) {
        return "menu";
      }
    }
    if (cv == "listingView") {
      return "menu";
    }
    if (cv == "listingView") {
      return "menu";
    }
    return "close";
  },
  isInvalidShare: () => {
    return shareInfo.isShare && !shareInfo.isValid;
  },
  isValidShare: () => {
    return shareInfo.isShare && shareInfo.isValid;
  },
};