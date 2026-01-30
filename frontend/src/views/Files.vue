<template>
  <div>
    <div v-if="loadingProgress < 100" class="progress-line" :style="{ width: loadingProgress + '%', ...moveWithSidebar }"></div>
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentViewLoaded" :is="currentView"></component>
    <div v-else>
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>
  </div>
</template>

<script>
import { filesApi, publicApi } from "@/api";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";
import OnlyOfficeEditor from "./files/OnlyOfficeEditor.vue";
import EpubViewer from "./files/EpubViewer.vue";
import DocViewer from "./files/DocViewer.vue";
import MarkdownViewer from "./files/MarkdownViewer.vue";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import router from "@/router";
import { extractSourceFromPath } from "@/utils/url";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

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
      // Share-specific data
      sharePassword: "",
      attemptedPasswordLogin: false,
      shareHash: null,
      shareSubPath: "",
      shareToken: "",
    };
  },
  computed: {
    share() {
      return state.share;
    },
    showShareInfo() {
      return getters.isShare() && state.isMobile && state.req.path == "/" && !state.shareInfo?.disableShareCard;
    },
    currentView() {
      return getters.currentView();
    },
    currentViewLoaded() {
      return getters.currentView() != "";
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

      mutations.showHover({
        name: "generic",
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
    async fetchData() {
      // Determine if this is a share based on current route
      const isShare = getters.isShare();
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
          const hash = getters.shareHash();
          let shareInfo = await publicApi.getShareInfo(hash);

          // Check if the response is an error
          if (!shareInfo || shareInfo.status >= 400) {
            this.error = {
              status: shareInfo?.status || "share404",
              message: shareInfo?.message || "errors.shareNotFound",
            };
            this.loadingProgress = 0;
            return;
          }

          // Valid share - add the hash and set shareInfo
          shareInfo.hash = hash;
          mutations.setShareInfo(shareInfo);

          // Parse share route to get shareHash and shareSubPath
          let urlPath = getters.routePath('public/share');
          let parts = urlPath.split("/");
          this.shareHash = parts[1];
          this.shareSubPath = "/" + parts.slice(2).join("/");

          // Check for password requirement
          if (shareInfo.hasPassword) {
            if (this.sharePassword === "") {
              this.sharePassword = localStorage.getItem("sharepass:" + this.shareHash);
              if (this.sharePassword === null || this.sharePassword === "") {
                this.showPasswordPrompt();
                return;
              }
            }
            localStorage.setItem("sharepass:" + this.shareHash, this.sharePassword);
          }

          if (shareInfo.themeColor) {
            document.documentElement.style.setProperty("--primaryColor", shareInfo.themeColor);
          }

          if (this.sharePassword === null) {
            this.sharePassword = "";
          }

          // For upload shares, validate password and return early
          if (shareInfo.shareType == "upload") {
            if (shareInfo.hasPassword) {
              mutations.setShareData({ passwordValid: false });
              try {
                await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, false, false);
                mutations.setShareData({ passwordValid: true });
                this.error = null;
              } catch (e) {
                if (e.status === 501) {
                  // 501 means browsing disabled - password is valid
                  mutations.setShareData({ passwordValid: true });
                  this.error = null;
                } else if (e.status === 401) {
                  this.attemptedPasswordLogin = true;
                  mutations.setShareData({ passwordValid: false });
                  this.showPasswordPrompt();
                  return;
                } else {
                  throw e;
                }
              }
            } else {
              mutations.setShareData({ passwordValid: true });
              this.error = null;
            }
            return;
          }

          // For regular shares, validate password
          if (shareInfo.hasPassword) {
            mutations.setShareData({ passwordValid: false });
            try {
              await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, false, false);
              mutations.setShareData({ passwordValid: true });
              this.error = null;
            } catch (e) {
              if (e.status === 401) {
                this.attemptedPasswordLogin = true;
                mutations.setShareData({ passwordValid: false });
                this.showPasswordPrompt();
                return;
              } else {
                throw e;
              }
            }
          } else {
            mutations.setShareData({ passwordValid: true });
            this.error = null;
          }

          mutations.resetSelected();
          mutations.setMultiple(false);

          if (state.shareInfo?.singleFileShare) {
            mutations.setSidebarVisible(true);
          }
        }

        // === FILES-SPECIFIC INITIALIZATION ===
        else {
          if (!getters.isLoggedIn()) {
            return;
          }

          mutations.clearShareData();
          const routePath = url.removeTrailingSlash(getters.routePath());

          // Redirect if multiple sources and user went to /files/
          if (routePath == "/files") {
            let targetPath = `/files/${state.sources.current}`;
            for (const link of state.user?.sidebarLinks || []) {
              if (link.target.startsWith('/')) {
                if (link.category !== 'source') {
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
            this.error = { message: $t("index.noSources") };
            mutations.replaceRequest({});
            return;
          }

          this.lastHash = "";
          mutations.resetSelected();
        }
        this.loadingProgress = 10;
        let file;
        if (isShare) {
          file = await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, false, false);
          file.hash = this.shareHash;
          this.shareToken = file.token;
          mutations.setShareData({
            hash: this.shareHash,
            token: this.shareToken,
            subPath: this.shareSubPath,
            passwordValid: true,
          });
        } else {
          const result = extractSourceFromPath(getters.routePath());
          const fetchSource = decodeURIComponent(result.source);
          const fetchPath = decodeURIComponent(result.path);
          file = await filesApi.fetchFiles(fetchSource, fetchPath, false, false);
        }

        // For non-directory files, fetch content if needed
        if (file.type !== "directory") {
          const content = !getters.fileViewingDisabled(file.name);

          if (content) {
            const contentFile = isShare
              ? await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, true, false)
              : await filesApi.fetchFiles(file.source, file.path, true, false);

            file = contentFile;

            if (isShare) {
              file.hash = this.shareHash;
              this.shareToken = contentFile.token;
            }
          }
        }

        // Set current source for multi-source setups (files only)
        if (!isShare && state.sources.count > 1) {
          mutations.setCurrentSource(file.source);
        }

        // Display first pass data immediately
        mutations.replaceRequest(file);
        document.title = `${document.title} - ${file.name}`;
        this.loadingProgress = 50;

        // === SECOND PASS: Fetch metadata in background (directories only) ===
        if (file.type === "directory" && file.hasMetadata) {
          this.loadingProgress = 90;

          // Fetch with metadata enabled (background operation)
          const metadataPromise = isShare
            ? publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, false, true)
            : filesApi.fetchFiles(file.source, file.path, false, true);

          metadataPromise
            .then(fileWithMetadata => {
              // Add share-specific properties if needed
              if (isShare) {
                fileWithMetadata.hash = this.shareHash;
                fileWithMetadata.token = this.shareToken;
              }

              // Capture scroll position before update
              const scrollY = window.scrollY;

              // Update with metadata-enriched data
              mutations.replaceRequest(fileWithMetadata);

              // Complete progress
              this.loadingProgress = 100;

              // Restore scroll position
              requestAnimationFrame(() => {
                window.scrollTo(0, scrollY);
              });
            })
            .catch(() => {
              // Don't throw - we already have the basic data displayed
              // Just complete the progress bar
              this.loadingProgress = 100;
            });
        } else {
          // No metadata needed, complete immediately
          this.loadingProgress = 100;
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
          this.attemptedPasswordLogin = this.sharePassword !== "";
          mutations.setShareData({ passwordValid: false });
          if (state.shareInfo?.shareType === "upload") {
            this.error = null;
          }
          this.showPasswordPrompt();
        } else {
          router.push({ name: "error" });
        }
      } finally {
        mutations.setLoading(isShare ? "share" : "files", false);
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
      mutations.showHover({
        name: "password",
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
        if (!getters.currentPromptName()) {
          mutations.showHover("help"); // Use mutation
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
          mutations.showHover({
            name: "rename",
            props: {
              item: state.req,
              parentItems: parentItems
            },
          });
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
