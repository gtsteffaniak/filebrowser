<template>
  <div>
    <div v-if="loadingProgress < 100" class="progress-line" :style="{ width: `${loadingProgress}%`, ...moveWithSidebar }"></div>
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentViewLoaded" :is="currentView" :fbdata="req"></component>
    <div v-else>
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>
  </div>
</template>

<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { resourcesApi, shareApi, mediaApi } from "@/api";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import { state, mutations, getters } from "@/store";
import { router } from "@/router";
import { extractSourceFromPath, removeLastDir, base64Encode, removeTrailingSlash } from "@/utils/url.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { globalVars } from "@/utils/constants";
import { syncEventTheme } from "@/utils/theme";
import { isRichTextPreviewMimeType } from "@/utils/mimetype";
import { invalidateDirMetadataCache } from "@/utils/metadataCache.ts";
import { showShareDownloadPrompt as openShareDownloadPrompt } from "@/utils/download.js";

function directoryListingHasMediaChildren(req) {
  return (
    req?.type === "directory" &&
    req.items?.some((i) => i.type?.startsWith("audio") || i.type?.startsWith("video"))
  );
}

const LISTING_CACHE_TTL_MS = 300000;
const LISTING_CACHE_MAX = 8;
const listingCache = new Map();

const trimTrailingSlash = (p) => (p || "/").replace(/\/+$/, "") || "/";

function listingCacheKey(source, path) {
  return `${state.user?.username}\n${source}\n${trimTrailingSlash(path)}`;
}

function getCachedListing(key) {
  const entry = listingCache.get(key);
  if (!entry) {
    return null;
  }
  if (Date.now() - entry.time > LISTING_CACHE_TTL_MS) {
    listingCache.delete(key);
    return null;
  }
  return entry;
}

function cacheListing(key, res) {
  listingCache.delete(key);
  listingCache.set(key, { res, time: Date.now() });
  if (listingCache.size > LISTING_CACHE_MAX) {
    listingCache.delete(listingCache.keys().next().value);
  }
}

function sameListing(a, b) {
  if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) {
    return false;
  }
  const describe = (i) => `${i.type}|${i.size}|${i.modified}|${i.pinned}|${i.hasPreview}`;
  const known = new Map();
  for (const item of a) {
    known.set(item.name, describe(item));
  }
  return b.every((item) => known.get(item.name) === describe(item));
}

function reusableParentItems(inScope, filePath) {
  const req = state.req;
  if (!req || !inScope(req)) {
    return null;
  }
  const items = req.type === "directory" ? req.items : req.parentDirItems;
  if (!Array.isArray(items)) {
    return null;
  }
  const target = trimTrailingSlash(filePath);
  return items.some((item) => trimTrailingSlash(item.path) === target) ? items : null;
}

/** @returns {Promise<{ items?: object[], name: string, type: string, path: string, source: string, hash?: string, token?: string, parentDirItems?: object[] }>} */
async function fetchShareItemWithParent(sharePassword, signal, canReuseParent = false) {
  let file = await resourcesApi.fetchFilesPublic(
    state.shareInfo.subPath,
    state.shareInfo.hash,
    sharePassword,
    false,
    false,
    false,
    signal
  );
  file.hash = state.shareInfo.hash;
  mutations.setShareData({ passwordValid: true });

  if (file.type === "directory") {
    return file;
  }

  let directoryPath = removeLastDir(state.shareInfo.subPath);
  if (!directoryPath || directoryPath === "") {
    directoryPath = "/";
  }
  const shouldFetchParent = directoryPath !== state.shareInfo.subPath;
  const cachedParent = shouldFetchParent && canReuseParent
    ? reusableParentItems((req) => req.hash === state.shareInfo.hash, file.path)
    : null;
  const contentPromise = getters.shouldFetchFileContent(file)
    ? resourcesApi
        .fetchFilesPublic(
          state.shareInfo.subPath,
          state.shareInfo.hash,
          sharePassword,
          true,
          false,
          false,
          signal
        )
        .catch(() => file)
    : Promise.resolve(file);
  const promises = [contentPromise];
  if (shouldFetchParent && !cachedParent) {
    promises.push(
      resourcesApi
        .fetchFilesPublic(directoryPath, state.shareInfo.hash, sharePassword, false, false, false, signal)
        .catch(() => null)
    );
  }
  const results = await Promise.all(promises);
  file = results[0];
  file.hash = state.shareInfo.hash;
  if (cachedParent) {
    file.parentDirItems = cachedParent;
  } else if (shouldFetchParent && results[1]?.items) {
    file.parentDirItems = results[1].items;
  }
  return file;
}

/** @returns {Promise<{ items?: object[], name: string, type: string, path: string, source: string, parentDirItems?: object[] }>} */
async function fetchAuthItemWithParent(fetchSource, fetchPath, canReuseParent = false, signal) {
  let res = await resourcesApi.fetchFiles(fetchSource, fetchPath, false, false, false, signal);
  if (res.type === "directory") {
    return res;
  }
  let directoryPath = removeLastDir(res.path);
  if (!directoryPath || directoryPath === "") {
    directoryPath = "/";
  }
  const shouldFetchParent = directoryPath !== res.path;
  const cachedParent = shouldFetchParent && canReuseParent
    ? reusableParentItems((req) => req.source === res.source, res.path)
    : null;
  const contentPromise = getters.shouldFetchFileContent(res)
    ? resourcesApi.fetchFiles(res.source, res.path, true, false, false, signal).catch(() => res)
    : Promise.resolve(res);
  const promises = [contentPromise];
  if (shouldFetchParent && !cachedParent) {
    promises.push(
      resourcesApi.fetchFiles(res.source, directoryPath, false, false, false, signal).catch(() => null)
    );
  }
  const results = await Promise.all(promises);
  res = results[0];
  if (cachedParent) {
    res.parentDirItems = cachedParent;
  } else if (shouldFetchParent && results[1]?.items) {
    res.parentDirItems = results[1].items;
  }
  return res;
}

export default {
  name: "files",
  components: {
    Errors,
    Preview,
    ListingView,
    LoadingSpinner,
    Editor: createAsyncComponent(() => import('@/views/files/Editor.vue')),
    OnlyOfficeEditor: createAsyncComponent(() => import('@/views/files/OnlyOfficeEditor.vue')),
    EpubViewer: createAsyncComponent(() => import('@/views/files/EpubViewer.vue')),
    DocViewer: createAsyncComponent(() => import('@/views/files/DocViewer.vue')),
    MarkdownViewer: createAsyncComponent(() => import('@/views/files/MarkdownViewer.vue')),
    ThreeJsViewer: createAsyncComponent(() => import('@/views/files/ThreeJs.vue')),
  },
  data() {
    return {
      error: null,
      width: window.innerWidth,
      lastPath: "",
      lastHash: "",
      popupSource: "",
      loadingProgress: 0,
      loadingStartTime: null,
      loadingTimeout: null,
      // Share-specific data
      sharePassword: "",
      attemptedPasswordLogin: false,
      lastShareHash: "",
    };
  },
  computed: {
    showShareInfo() {
      return getters.isShare() && getters.isMobile() && state.req.path === "/" && !state.shareInfo?.disableShareCard;
    },
    currentView() {
      return getters.currentView();
    },
    currentViewLoaded() {
      return getters.currentView() !== "";
    },
    req() {
      return state.req;
    },
    reload() {
      return state.reload;
    },
    moveWithSidebar() {
      const style = {
        width: `${this.loadingProgress}%`,
      };
      if (getters.isStickySidebar() && getters.isSidebarVisible()) {
        style.left = `${state.sidebar.width}em`;
      }
      return style;
    },
  },
  created() {
    this.fetchId = 0;
    this.loadingFetchId = 0;
    this.fetchController = null;
    const isHalloween = getters.eventTheme() === "halloween";
    syncEventTheme(isHalloween);
    if (isHalloween && !localStorage.getItem("seenHalloweenMessage")) {
      mutations.showPrompt({
        name: "generic",
        pinned: true,
        props: {
          title: this.$t("prompts.halloweenTitle"),
          body: this.$t("prompts.halloweenBody"),
          buttons: [
            {
              label: this.$t("general.close"),
              action: () => {
                localStorage.setItem("seenHalloweenMessage", "true");
                mutations.closeTopPrompt();
              },
            },
            {
              label: this.$t("general.disable"),
              action: () => {
                mutations.disableEventThemes();
                localStorage.setItem("seenHalloweenMessage", "true");
                mutations.closeTopPrompt();
              },
              primary: true,
            },
          ],
        },
      });
    }
    this.fetchData();
  },
  watch: {
    $route() {
      syncEventTheme(getters.eventTheme() === "halloween");
      this.fetchData();
    },
    reload(value) {
      if (value) {
        this.fetchData();
      }
    },
  },
  mounted() {
    window.addEventListener("hashchange", this.scrollToHash);
    window.addEventListener("keydown", this.keyEvent);
  },
  beforeUnmount() {
    this.fetchId++;
    this.fetchController?.abort();
    window.removeEventListener("hashchange", this.scrollToHash);
    window.removeEventListener("keydown", this.keyEvent);
  },
  unmounted() {
    mutations.replaceRequest({}); // Use mutation
  },
  methods: {
    scrollToHash() {
      let scrollToId = "";
      let targetName = "";
      // scroll to previous item either from location hash or from previousItemHashId state
      // prefers location hash
      const noHashChange = window.location.hash === this.lastHash
      if (noHashChange && state.previousHistoryItem?.name === "") return;
      this.lastHash = window.location.hash;
      if (window.location.hash) {
        const rawHash = window.location.hash.slice(1);
        let decodedName;
        try {
          decodedName = decodeURIComponent(rawHash);
        } catch (_e) {
          // If the hash contains malformed escape sequences, fall back to raw
          decodedName = rawHash;
        }
        targetName = decodedName;
        scrollToId = base64Encode(encodeURIComponent(decodedName));
      } else if (state.previousHistoryItem?.name &&
                 state.previousHistoryItem.path === state.req.path &&
                 state.previousHistoryItem.source === state.req.source) {
        targetName = state.previousHistoryItem?.name;
        scrollToId = base64Encode(encodeURIComponent(state.previousHistoryItem?.name));
      }
      // Don't call getElementById with empty string
      if (!scrollToId || scrollToId.trim() === '') {
        return;
      }
      // Re-select the item we are returning to -- replaces the previous glow effect.
      const targetIndex = state.req?.items?.findIndex((item) => item.name === targetName);
      if (typeof targetIndex === 'number' && targetIndex !== -1) {
        mutations.addSelected(targetIndex);
      }
      const element = document.getElementById(scrollToId);
      if (element) {
        element.scrollIntoView({
          behavior: "instant",
          block: "center",
        });
      }
    },
    async patchMediaMetadataIfNeeded(listing, source) {
      if (!directoryListingHasMediaChildren(listing)) {
        this.loadingProgress = 100;
        return;
      }
      this.loadingProgress = 90;
      const path = listing.path;
      const currentSource = listing.source ?? source;
      const currentHash = state.shareInfo?.hash;
      const isShare = getters.isShare();
      const isCurrentListing = () => isShare
        ? getters.isShare() && state.shareInfo?.hash === currentHash && state.req?.path === path
        : !getters.isShare() && state.req?.source === currentSource && state.req?.path === path;
      try {
        const metaMap = isShare
          ? await mediaApi.getDirectoryMetadataMap(state.shareInfo.subPath, {
              isShare: true,
              hash: state.shareInfo.hash,
              password: this.sharePassword,
            })
          : await mediaApi.getDirectoryMetadataMap(path, { source: currentSource });
        if (!isCurrentListing()) {
          return;
        }
        if (state.req.items) {
          mutations.patchListingMetadata(state.req.items, metaMap);
        }
        this.loadingProgress = 100;
      } catch {
        if (isCurrentListing()) {
          this.loadingProgress = 0;
        }
      }
    },

    async fetchData() {
      const hash = getters.shareHash();
      const isShare = hash !== "";
      const fetchId = ++this.fetchId;
      const isStale = () => fetchId !== this.fetchId;
      this.fetchController?.abort();
      const controller = new AbortController();
      this.fetchController = controller;
      const signal = controller.signal;

      // Fetch and store share info if this is a share
      if (isShare) {
        let shareInfo;
        try {
          shareInfo = await shareApi.getShareInfoPublic(hash, signal);
        } catch (e) {
          if (isStale()) return;
          throw e;
        }
        if (isStale()) return;

        // Check if the response is an error (has status field indicating error)
        if (!shareInfo || shareInfo.status >= 400) {
          // show message that share is invalid and don't do anything else
          this.error = {
            status: shareInfo?.status || "share404",
            message: shareInfo?.message || "errors.shareNotFound",
          };
          this.loadingProgress = 0;
          return;
        }

        // Valid share - add the hash and other required fields, then store in state
        shareInfo.hash = hash;

        const sameShare =
          this.lastShareHash.length === hash.length &&
          this.lastShareHash.indexOf(hash) === 0;
        if (this.lastShareHash && !sameShare) {
          this.sharePassword = "";
        }
        this.lastShareHash = hash;

        // Parse share route to get subPath
        const urlPath = getters.routePath('public/share')
        const parts = urlPath.split("/");
        // Decode each part since URL paths are encoded
        const decodedParts = parts.slice(2).map(part => decodeURIComponent(part));
        shareInfo.subPath = `/${decodedParts.join("/")}`;
        // Set shareInfo in state
        mutations.setShareInfo(shareInfo);

        // Check for password requirement (applies to both regular and upload shares)
        if (shareInfo.hasPassword) {
          if (this.sharePassword === "") {
            this.sharePassword = localStorage.getItem(`sharepass:${shareInfo.hash}`);
            if (this.sharePassword === null || this.sharePassword === "") {
              this.showPasswordPrompt();
              return;
            }
          }
        }

        if (shareInfo.themeColor) {
          document.documentElement.style.setProperty("--primaryColor", shareInfo.themeColor);
        }

        // Handle password (same for both regular and upload shares)
        if (this.sharePassword === null) {
          this.sharePassword = "";
        }
      }

      if (state.deletedItem) {
        return;
      }

      if (!state.user.sorting) {
        mutations.updateListingSortConfig({
          field: "name",
          asc: true,
        });
      }

      // keep the list mounted
      const wasReload = state.reload;
      const keepList =
        wasReload &&
        state.req?.type === "directory" &&
        Array.isArray(state.req.items) &&
        this.lastPath === state.route.path;
      if (!keepList) {
        mutations.setLoading(isShare ? "share" : "files", true);
        this.loadingFetchId = fetchId;
      }
      this.error = null;
      mutations.setReload(false);
      if (wasReload) {
        listingCache.clear();
      }
      let cacheKey = null;
      let skipTailScroll = wasReload;

      try {
        if (isShare) {
          // For upload shares, validate password on startup and return early
          // Password validation happens via fetchPub call, which will throw 401 if incorrect
          // A 501 error means browsing is disabled (expected for upload shares) and indicates auth succeeded
          if (state.shareInfo.shareType === "upload") {
            // Initialize password validation state
            if (state.shareInfo.hasPassword) {
              mutations.setShareData({ passwordValid: false });
              try {
                await resourcesApi.fetchFilesPublic(state.shareInfo.subPath, state.shareInfo.hash, this.sharePassword, false, false, false, signal);
                if (isStale()) return;
                // If we get here, password is valid (unlikely for upload shares, but handle it)
                mutations.setShareData({ passwordValid: true });
                this.error = null; // Clear any previous errors
              } catch (e) {
                if (isStale()) return;
                // 501 means browsing is disabled for upload shares - this is expected and means auth succeeded
                if (e.status === 501) {
                  mutations.setShareData({ passwordValid: true });
                  this.error = null; // Clear any previous errors
                } else {
                  throw e;
                }
              }
            } else {
              // No password required, mark as validated
              mutations.setShareData({ passwordValid: true });
              this.error = null; // Clear any previous errors
            }
            return;
          }

          // For regular shares, validate password on startup (similar to upload shares)
          if (state.shareInfo.hasPassword) {
            mutations.setShareData({ passwordValid: false });
            await resourcesApi.fetchFilesPublic(state.shareInfo.subPath, state.shareInfo.hash, this.sharePassword, false, false, false, signal);
            if (isStale()) return;
            mutations.setShareData({ passwordValid: true });
            this.error = null; // Clear any previous errors
          } else {
            // No password required, mark as validated
            mutations.setShareData({ passwordValid: true });
            this.error = null; // Clear any previous errors
          }

          mutations.resetSelected();
          mutations.setMultiple(false);

          if (state.shareInfo?.singleFileShare) {
            mutations.setSidebarVisible(true);
          }

          this.loadingProgress = 10;
          const file = await fetchShareItemWithParent(this.sharePassword, signal, !wasReload);
          if (isStale()) return;
          mutations.replaceRequest(file);
          document.title = `${globalVars.name} - ${this.$t("general.share")} - ${file.name}`;
          await this.patchMediaMetadataIfNeeded(file);
          if (isStale()) return;
          if (this.$route.query.download === "true") {
            this.promptShareDownload(file);
          }
        }

        // === FILES-SPECIFIC INITIALIZATION ===
        else {
          if (!getters.isLoggedIn()) {
            return;
          }

          // Clear share data when accessing files
          mutations.clearShareData();
          const routePath = removeTrailingSlash(getters.routePath());

          // Redirect if multiple sources and user went to /files/
          if (routePath === "/files") {
            let targetPath = `/${state.sources.current}`;
            for (const link of state.user?.sidebarLinks || []) {
              if (link.target.startsWith('/')) {
                if (!link.category.startsWith('source')) {
                  continue;
                }
                targetPath = `/${link.sourceName}${link.target}`;
                break;
              }
            }
            void router.push(targetPath);
            return;
          }

          const result = extractSourceFromPath(getters.routePath(), Object.keys(state.sources.info));

          if (result.source === "") {
            // No sources available - show a more graceful message instead of error popup
            this.error = { status: "noSources" };
            mutations.replaceRequest({});
            return;
          }

          this.lastHash = "";
          mutations.resetSelected();

          this.loadingProgress = 10;
          const fetchSource = decodeURIComponent(result.source);
          const fetchPath = decodeURIComponent(result.path);

          // Show the last listing of the folder and refresh it in the background
          cacheKey = listingCacheKey(fetchSource, fetchPath);
          const cached = wasReload ? null : getCachedListing(cacheKey);
          if (cached) {
            if (state.sources.count > 1) {
              mutations.setCurrentSource(cached.res.source);
            }
            document.title = `${globalVars.name} - ${this.$t("general.files")} - ${cached.res.name}`;
            mutations.replaceRequest(cached.res);
            mutations.setLoading("files", false);
            if (state.navigation.isTransitioning) {
              mutations.setNavigationTransitioning(false);
            }
            setTimeout(() => {
              this.scrollToHash();
            }, 25);
          }

          const res = await fetchAuthItemWithParent(fetchSource, fetchPath, !wasReload, signal);
          if (isStale()) return;
          if (state.sources.count > 1) {
            mutations.setCurrentSource(res.source);
          }
          document.title = `${globalVars.name} - ${this.$t("general.files")} - ${res.name}`;
          if (cached && res.type === "directory" && sameListing(state.req?.items, res.items)) {
            cacheListing(cacheKey, cached.res);
            skipTailScroll = true;
            await this.patchMediaMetadataIfNeeded(state.req, fetchSource);
          } else {
            if (cached) {
              this.lastHash = "";
            }
            mutations.replaceRequest(res);
            mutations.setLoading("files", false);
            if (res.type === "directory") {
              cacheListing(cacheKey, res);
            } else {
              listingCache.delete(cacheKey);
            }
            await this.patchMediaMetadataIfNeeded(res, fetchSource);
          }
        }

      } catch (e) {
        if (isStale()) return;
        if (cacheKey) {
          listingCache.delete(cacheKey);
        }
        this.error = e;
        mutations.replaceRequest({});
        this.loadingProgress = 0;
        if (e.status === 401 && isShare) {
          // Handle share password requirement
          this.clearStoredSharePassword(state.shareInfo?.hash);
          this.attemptedPasswordLogin = this.sharePassword !== "";
          // Reset password validation state on wrong password
          mutations.setShareData({ passwordValid: false });
          // Clear error for upload shares so upload interface can be shown once password is correct
          if (state.shareInfo?.shareType === "upload") {
            this.error = null;
          }
          this.showPasswordPrompt();
        }
      } finally {
        if (this.loadingFetchId === fetchId) {
          this.loadingFetchId = 0;
          mutations.setLoading(isShare ? "share" : "files", false);
          // Clear navigation transition when data fetch completes
          if (state.navigation.isTransitioning) {
            mutations.setNavigationTransitioning(false);
          }
        }
      }
      if (isStale()) return;
      if (!skipTailScroll) {
        setTimeout(() => {
          this.scrollToHash();
        }, 25);
      }
      this.lastPath = state.route.path;
    },

    storeSharePassword(hash, password) {
      if (!hash || !password) {
        return;
      }
      localStorage.setItem(`sharepass:${hash}`, password);
    },
    clearStoredSharePassword(hash) {
      if (!hash) {
        return;
      }
      localStorage.removeItem(`sharepass:${hash}`);
    },
    showPasswordPrompt() {
      mutations.showPrompt({
        name: "password",
        pinned: true,
        props: {
          submitCallback: (password) => {
            this.sharePassword = password;
            this.storeSharePassword(state.shareInfo?.hash, password);
            void this.fetchData();
          },
          showWrongCredentials: this.attemptedPasswordLogin,
          initialPassword: this.sharePassword,
        },
      });
    },
    resolveShareDownloadPath(file) {
      const subPath = state.shareInfo?.subPath;
      if (subPath && subPath !== "/") {
        return subPath;
      }
      if (file?.path && file.path !== "/") {
        return file.path;
      }
      return "/";
    },
    promptShareDownload(file) {
      const downloadPath = this.resolveShareDownloadPath(file);
      openShareDownloadPrompt([{
        path: downloadPath,
        name: file?.name,
        type: file?.type,
        size: file?.size,
        isDir: file?.type === "directory",
      }]);
    },
    keyEvent(event) {
      // F1!
      if (event.key === "F1" && !event.ctrlKey && !event.metaKey) {
        event.preventDefault();
        mutations.setSearch(false);
        if (!getters.currentPromptName()) {
          mutations.showPrompt("help"); // Use mutation
        }
      }

      // Ctrl+, - navigate to settings
      if (event.ctrlKey && event.key === ',') {
        event.preventDefault();
        void router.push('/settings');
      }

      // Esc! - for shares, reset selection
      if (getters.isShare() && event.key === "Escape") {
        if (getters.selectedCount() > 0) {
          mutations.resetSelected();
        }
      }
      // F2! - for rename in listing or preview
      if (event.key === "F2" && !event.ctrlKey && !event.metaKey) {
        event.preventDefault();
        if (getters.currentPromptName()) return;

        const canModify = getters.sourcePermissions().modify;

        if (getters.isPreviewView() && canModify) {
          const parentItems = state.navigation.listing || [];
          mutations.showPrompt({
            name: "rename",
            props: {
              item: state.req,
              parentItems: parentItems
            },
          });
        } else if (getters.currentView() === 'listingView' && canModify && state.selected.length === 1) {
          const item = getters.getFirstSelected();
          if (item) {
            mutations.showPrompt({ name: "rename", props: { item, parentItems: [] } });
          }
        }
      }
      // F4! - refresh the listing with its metadata
      if (event.key === "F4" && !event.ctrlKey && !event.metaKey && !event.repeat) {
        if (getters.currentPromptName() || getters.currentView() !== 'listingView') return;
        event.preventDefault();
        invalidateDirMetadataCache();
        mutations.setReload(true);
      }
      // CTRL+E - switch between editor and markdown viewer
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'e') {
        event.preventDefault();
        const currentFile = state.req;
        if (isRichTextPreviewMimeType(currentFile?.type)) {
          const currentView = getters.currentView();
          if (currentView === 'editor') {
            void router.replace({ hash: '#preview' });
          } else if (currentView === 'markdownViewer') {
            if (getters.sourcePermissions()?.modify) {
              void router.replace({ hash: '#edit' });
            }
          }
        }
      }
      // Ctrl+F1, Ctrl+F2, Ctrl+F3 to change view modes
      if ((event.ctrlKey || event.metaKey) && ['F1', 'F2', 'F3'].includes(event.key)) {
        if (getters.currentView() !== 'listingView') return;
        event.preventDefault();

        const key = event.key;
        let baseMode;
        switch (key) {
          case 'F1':
            baseMode = 'normal';
            break;
          case 'F2':
            baseMode = 'icons';
            break;
          case 'F3':
            baseMode = 'list';
            break;
          default: return;
        }
        const size = state.user?.gallerySize ?? 5;
        let newViewMode = baseMode;
        if (baseMode === 'list') {
          newViewMode = size <= 3 ? 'compact' : 'list';
        } else if (baseMode === 'icons') {
          newViewMode = size <= 4 ? 'icons' : 'gallery';
        }

        mutations.updateDisplayPreferences({ viewMode: newViewMode });
        void mutations.updateCurrentUser({ viewMode: newViewMode });
        mutations.closeHovers();
      }
    },
  },
};
</script>

<style>
.share-info-component {
  margin-top: 0.5em;
}

.progress-line {
  position: fixed;
  top: 4em;
  left: 0;
  right: 0;
  height: 1px;
  background: var(--primaryColor);
  z-index: 2000;
  transition: width 0.3s ease, left 0.2s ease;
  box-shadow: 0 0 10px var(--primaryColor);
}

</style>
