<template>
  <div>
    <div v-if="loadingProgress < 100" class="progress-line" :style="{ width: loadingProgress + '%', ...moveWithSidebar }"></div>
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
import { resourcesApi, shareApi, mediaApi } from "@/api";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";
import OnlyOfficeEditor from "./files/OnlyOfficeEditor.vue";
import EpubViewer from "./files/EpubViewer.vue";
import DocViewer from "./files/DocViewer.vue";
import MarkdownViewer from "./files/MarkdownViewer.vue";
import ThreeJsViewer from "./files/ThreeJs.vue";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import router from "@/router";
import { extractSourceFromPath } from "@/utils/url";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

function directoryListingHasMediaChildren(req) {
  return (
    req?.type === "directory" &&
    req.items?.some((i) => i.type?.startsWith("audio") || i.type?.startsWith("video"))
  );
}

/** @returns {Promise<{ items?: object[], name: string, type: string, path: string, source: string, hash?: string, token?: string, parentDirItems?: object[] }>} */
async function fetchShareItemWithParent(sharePassword) {
  let file = await resourcesApi.fetchFilesPublic(
    state.shareInfo.subPath,
    state.shareInfo.hash,
    sharePassword,
    false,
    false
  );
  file.hash = state.shareInfo.hash;
  mutations.setShareData({ token: file.token, passwordValid: true });

  if (file.type === "directory") {
    return file;
  }

  const content = !getters.fileViewingDisabled(file.name);
  let directoryPath = url.removeLastDir(state.shareInfo.subPath);
  if (!directoryPath || directoryPath === "") {
    directoryPath = "/";
  }
  const shouldFetchParent = directoryPath !== state.shareInfo.subPath;
  const promises = [
    resourcesApi.fetchFilesPublic(
      state.shareInfo.subPath,
      state.shareInfo.hash,
      sharePassword,
      content,
      false
    ),
  ];
  if (shouldFetchParent) {
    promises.push(
      resourcesApi
        .fetchFilesPublic(directoryPath, state.shareInfo.hash, sharePassword, false, false)
        .catch(() => null)
    );
  }
  const results = await Promise.all(promises);
  file = results[0];
  file.hash = state.shareInfo.hash;
  mutations.setShareData({ token: results[0].token });
  if (shouldFetchParent && results[1]?.items) {
    file.parentDirItems = results[1].items;
  }
  return file;
}

/** @returns {Promise<{ items?: object[], name: string, type: string, path: string, source: string, parentDirItems?: object[] }>} */
async function fetchAuthItemWithParent(fetchSource, fetchPath) {
  let res = await resourcesApi.fetchFiles(fetchSource, fetchPath, false, false);
  if (res.type === "directory" || res.type.startsWith("image")) {
    return res;
  }
  const content = !getters.fileViewingDisabled(res.name);
  let directoryPath = url.removeLastDir(res.path);
  if (!directoryPath || directoryPath === "") {
    directoryPath = "/";
  }
  const shouldFetchParent = directoryPath !== res.path;
  const promises = [resourcesApi.fetchFiles(res.source, res.path, content, false)];
  if (shouldFetchParent) {
    promises.push(
      resourcesApi.fetchFiles(res.source, directoryPath, false, false).catch(() => null)
    );
  }
  const results = await Promise.all(promises);
  res = results[0];
  if (shouldFetchParent && results[1]?.items) {
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
    Editor,
    EpubViewer,
    DocViewer,
    OnlyOfficeEditor,
    MarkdownViewer,
    ThreeJsViewer,
    LoadingSpinner,
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
    };
  },
  computed: {
    showShareInfo() {
      return getters.isShare() && state.isMobile && state.req.path == "/" && !state.shareInfo?.disableShareCard;
    },
    currentView() {
      return getters.currentView();
    },
    currentViewLoaded() {
      return getters.currentView() != "";
    },
    req() {
      return state.req;
    },
    reload() {
      return state.reload;
    },
    moveWithSidebar() {
      const style = {
        width: this.loadingProgress + '%',
      };
      if (getters.isStickySidebar() && getters.isSidebarVisible()) {
        style.left = state.sidebar.width + 'em';
      }
      return style;
    },
  },
  created() {
    if (getters.eventTheme() === "halloween" && !localStorage.getItem("seenHalloweenMessage")) {
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
              },
            },
            {
              label: this.$t("general.disable"),
              action: () => {
                mutations.disableEventThemes();
                localStorage.setItem("seenHalloweenMessage", "true");
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
    $route: "fetchData",
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
    window.removeEventListener("keydown", this.keyEvent);
  },
  unmounted() {
    mutations.replaceRequest({}); // Use mutation
  },
  methods: {
    scrollToHash() {
      let scrollToId = "";
      // scroll to previous item either from location hash or from previousItemHashId state
      // prefers location hash
      const noHashChange = window.location.hash === this.lastHash
      if (noHashChange && state.previousHistoryItem?.name === "") return;
      this.lastHash = window.location.hash;
      if (window.location.hash) {
        const rawHash = window.location.hash.slice(1);
        let decodedName = rawHash;
        try {
          decodedName = decodeURIComponent(rawHash);
        } catch (e) {
          // If the hash contains malformed escape sequences, fall back to raw
          decodedName = rawHash;
        }
        scrollToId = url.base64Encode(encodeURIComponent(decodedName));

      } else if (state.previousHistoryItem?.name && state.previousHistoryItem.path === state.req.path && state.previousHistoryItem.source === state.req.source) {
        scrollToId = url.base64Encode(encodeURIComponent(state.previousHistoryItem.name));
      }
      // Don't call getElementById with empty string
      if (!scrollToId || scrollToId.trim() === '') {
        return;
      }
      const element = document.getElementById(scrollToId);
        if (element) {
          element.scrollIntoView({
            behavior: "instant",
            block: "center",
          });
          // Add glow effect
          element.classList.add('scroll-glow');
          // Remove glow effect after animation completes
          setTimeout(() => {
            element.classList.remove('scroll-glow');
          }, 1000);
        }
    },
    async patchMediaMetadataIfNeeded(listing, fetchMedia) {
      if (directoryListingHasMediaChildren(listing)) {
        this.loadingProgress = 90;
        try {
          const payload = await fetchMedia();
          if (payload?.items?.length) {
            mutations.patchRequestMetadata(payload.items);
          }
          this.loadingProgress = 100;
        } catch {
          this.loadingProgress = 0;
        }
        return;
      }
      this.loadingProgress = 100;
    },

    async fetchData() {
      const hash = getters.shareHash();
      let isShare = hash !== "";

      // Fetch and store share info if this is a share
      if (isShare) {
        let shareInfo = await shareApi.getShareInfoPublic(hash);

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

        // Parse share route to get subPath
        let urlPath = getters.routePath('public/share')
        let parts = urlPath.split("/");
        // Decode each part since URL paths are encoded
        let decodedParts = parts.slice(2).map(part => decodeURIComponent(part));
        shareInfo.subPath = "/" + decodedParts.join("/");
        // Set shareInfo in state
        mutations.setShareInfo(shareInfo);

        // Check for password requirement (applies to both regular and upload shares)
        if (shareInfo.hasPassword) {
          if (this.sharePassword === "") {
            this.sharePassword = localStorage.getItem("sharepass:" + shareInfo.hash);
            if (this.sharePassword === null || this.sharePassword === "") {
              this.showPasswordPrompt();
              return;
            }
          }
          // Store password in localStorage
          localStorage.setItem("sharepass:" + shareInfo.hash, this.sharePassword);
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

      // Set loading and reset error
      mutations.setLoading(isShare ? "share" : "files", true);
      this.error = null;
      mutations.setReload(false);

      try {
        if (isShare) {
          // For upload shares, validate password on startup and return early
          // Password validation happens via fetchPub call, which will throw 401 if incorrect
          // A 501 error means browsing is disabled (expected for upload shares) and indicates auth succeeded
          if (state.shareInfo.shareType == "upload") {
            // Initialize password validation state
            if (state.shareInfo.hasPassword) {
              mutations.setShareData({ passwordValid: false });
              try {
                await resourcesApi.fetchFilesPublic(state.shareInfo.subPath, state.shareInfo.hash, this.sharePassword, false, false);
                // If we get here, password is valid (unlikely for upload shares, but handle it)
                mutations.setShareData({ passwordValid: true });
                this.error = null; // Clear any previous errors
              } catch (e) {
                // 501 means browsing is disabled for upload shares - this is expected and means auth succeeded
                if (e.status === 501) {
                  // Password is valid, mark as validated
                  mutations.setShareData({ passwordValid: true });
                  this.error = null; // Clear any previous errors
                } else if (e.status === 401) {
                  // Password is invalid, show prompt
                  this.attemptedPasswordLogin = true;
                  mutations.setShareData({ passwordValid: false });
                  this.showPasswordPrompt();
                  return;
                } else {
                  // For other errors, re-throw to be handled by outer catch
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
            try {
              await resourcesApi.fetchFilesPublic(state.shareInfo.subPath, state.shareInfo.hash, this.sharePassword, false, false);
              // Password is valid
              mutations.setShareData({ passwordValid: true });
              this.error = null; // Clear any previous errors
            } catch (e) {
              if (e.status === 401) {
                // Password is invalid, show prompt
                this.attemptedPasswordLogin = true;
                mutations.setShareData({ passwordValid: false });
                this.showPasswordPrompt();
                return;
              } else {
                // For other errors, re-throw to be handled by outer catch
                throw e;
              }
            }
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
          const file = await fetchShareItemWithParent(this.sharePassword);
          mutations.replaceRequest(file);
          document.title = globalVars.name + " - " + this.$t("general.share") + " - " + file.name;
          await this.patchMediaMetadataIfNeeded(file, () =>
            mediaApi.fetchDirectoryMediaMetadataPublic(
              state.shareInfo.subPath,
              state.shareInfo.hash,
              this.sharePassword
            )
          );
        }

        // === FILES-SPECIFIC INITIALIZATION ===
        else {
          if (!getters.isLoggedIn()) {
            return;
          }

          // Clear share data when accessing files
          mutations.clearShareData();
          const routePath = url.removeTrailingSlash(getters.routePath());

          // Redirect if multiple sources and user went to /files/
          if (routePath == "/files") {
            let targetPath = `/files/${state.sources.current}`;
            for (const link of state.user?.sidebarLinks || []) {
              if (link.target.startsWith('/')) {
                if (!link.category.startsWith('source')) {
                  continue;
                }
                targetPath = `/files/${link.sourceName}${link.target}`;
                break;
              }
            }
            router.push(targetPath);
            return;
          }

          const result = extractSourceFromPath(getters.routePath());

          if (result.source === "") {
            // No sources available - show a more graceful message instead of error popup
            this.error = { message: $t("index.noSources") };
            mutations.replaceRequest({});
            return;
          }

          this.lastHash = "";
          mutations.resetSelected();

          this.loadingProgress = 10;
          const fetchSource = decodeURIComponent(result.source);
          const fetchPath = decodeURIComponent(result.path);

          const res = await fetchAuthItemWithParent(fetchSource, fetchPath);
          if (state.sources.count > 1) {
            mutations.setCurrentSource(res.source);
          }
          document.title = globalVars.name + " - " + this.$t("general.files") + " - " + res.name;
          mutations.replaceRequest(res);
          mutations.setLoading("files", false);
          await this.patchMediaMetadataIfNeeded(res, () =>
            mediaApi.fetchDirectoryMediaMetadata(fetchSource, fetchPath)
          );
        }

      } catch (e) {
        this.error = e;
        mutations.replaceRequest({});
        this.loadingProgress = 0;

        if (e.status === 404) {
          router.push({ name: "notFound" });
        } else if (e.status === 403) {
          router.push({ name: "forbidden" });
        } else if (e.status === 401 && isShare) {
          // Handle share password requirement
          this.attemptedPasswordLogin = this.sharePassword !== "";
          // Reset password validation state on wrong password
          mutations.setShareData({ passwordValid: false });
          // Clear error for upload shares so upload interface can be shown once password is correct
          if (state.shareInfo?.shareType === "upload") {
            this.error = null;
          }
          this.showPasswordPrompt();
        } else {
          router.push({ name: "error" });
        }
      } finally {
        mutations.setLoading(isShare ? "share" : "files", false);
        // Clear navigation transition when data fetch completes
        if (state.navigation.isTransitioning) {
          mutations.setNavigationTransitioning(false);
        }
      }

      setTimeout(() => {
        this.scrollToHash();
      }, 25);
      this.lastPath = state.route.path;
    },

    showPasswordPrompt() {
      mutations.showPrompt({
        name: "password",
        pinned: true,
        props: {
          submitCallback: (password) => {
            this.sharePassword = password;
            this.fetchData();
          },
          showWrongCredentials: this.attemptedPasswordLogin,
          initialPassword: this.sharePassword,
        },
      });
    },
    keyEvent(event) {
      // F1!
      if (event.key === "F1") {
        event.preventDefault();
        mutations.setSearch(false);
        if (!getters.currentPromptName()) {
          mutations.showPrompt("help"); // Use mutation
        }
      }

      // Ctrl+, - navigate to settings
      if (event.ctrlKey && event.key === ',') {
        event.preventDefault();
        router.push('/settings');
      }

      // Esc! - for shares, reset selection
      if ( getters.isShare() && event.key === "Escape") {
        if (getters.selectedCount() > 0) {
          mutations.resetSelected();
        }
      }
      // F2! - for rename in previews
      if (event.key == "F2" && getters.isPreviewView() && getters.permissions()?.modify) {
        event.preventDefault();
        if (!getters.currentPromptName()) {
          const parentItems = state.navigation.listing || [];
          mutations.showPrompt({
            name: "rename",
            props: {
              item: state.req,
              parentItems: parentItems
            },
          });
        }
      }
      // CTRL+E - switch between editor and markdown viewer
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'e') {
        event.preventDefault();
        const currentFile = state.req;
        if (currentFile && currentFile.type === 'text/markdown') {
          const currentView = getters.currentView();
          if (currentView === 'editor') {
            router.replace({ hash: '#preview' });
          } else if (currentView === 'markdownViewer') {
            if (getters.permissions()?.modify) {
              router.replace({ hash: '#edit' });
            }
          }
        }
      }
    },
  },
};
</script>

<style>
.scroll-glow {
  animation: scrollGlowAnimation 1s ease-out;
}

@keyframes scrollGlowAnimation {
  0% {
    color: inherit;
  }
  50% {
    color: var(--primaryColor);
  }
  100% {
    color: inherit;
  }
}

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
