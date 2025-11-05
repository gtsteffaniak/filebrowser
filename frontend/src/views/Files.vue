<template>
  <div>
    <!-- Share Info Component -->
    <ShareInfoCard
      v-if="showShareInfo"
      class="share-info-component"
      :hash="share?.hash"
      :token="share?.token"
      :subPath="share?.subPath"
    />

    <breadcrumbs v-if="showBreadCrumbs" :base="isShare ? `/share/${shareHash}` : undefined" />
    <errors v-if="error && !(isShare && error.status === 401)" :errorCode="error.status" />
    <component v-else-if="currentViewLoaded" :is="currentView"></component>
    <div v-else>
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ $t("general.loading") }}</span>
      </h2>
    </div>
  </div>
  <PopupPreview v-if="popupEnabled" />
</template>

<script>
import { filesApi, publicApi } from "@/api";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
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
import { globalVars, shareInfo } from "@/utils/constants";
import PopupPreview from "@/components/files/PopupPreview.vue";
import { extractSourceFromPath } from "@/utils/url";
import ShareInfoCard from "@/components/files/ShareInfoCard.vue";

export default {
  name: "files",
  components: {
    Breadcrumbs,
    Errors,
    Preview,
    ListingView,
    Editor,
    EpubViewer,
    DocViewer,
    OnlyOfficeEditor,
    MarkdownViewer,
    PopupPreview,
    ShareInfoCard,
  },
  data() {
    return {
      error: null,
      width: window.innerWidth,
      lastPath: "",
      lastHash: "",
      popupSource: "",
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
      return getters.isShare() && state.isMobile && state.req.path == "/" && !shareInfo.disableShareCard;
    },
    popupEnabled() {
      if (!state.user || state.user?.username == "") {
        return false;
      }
      return state.user.preview.popup;
    },
    showBreadCrumbs() {
      return getters.showBreadCrumbs();
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
    if (getters.isInvalidShare()) {
      // show message that share is invalid and don't do anything else
      this.error = {
        status: "share404",
        message: "errors.shareNotFound",
      };
    }
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
      if (noHashChange && state.previousHistoryItem.name === "") return;
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

      } else if (state.previousHistoryItem.name && state.previousHistoryItem.path === state.req.path && state.previousHistoryItem.source === state.req.source) {
        scrollToId = url.base64Encode(encodeURIComponent(state.previousHistoryItem.name));
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
      if (state.deletedItem || getters.isInvalidShare() || shareInfo.shareType == "upload") {
        return
      }

      if (!state.user.sorting) {
        mutations.updateListingSortConfig({
          field: "name",
          asc: true,
        });
      }
      // Set loading and reset error
      mutations.setLoading(getters.isShare() ? "share" : "files", true);
      this.error = null;
      mutations.setReload(false);

      try {
        if (getters.isShare()) {
          await this.fetchShareData();
        } else {
          await this.fetchFilesData();
        }
      } catch (e) {
        this.error = e;
        mutations.replaceRequest({});
        if (e.status === 404) {
          router.push({ name: "notFound" });
        } else if (e.status === 403) {
          router.push({ name: "forbidden" });
        } else if (e.status === 401 && getters.isShare()) {
          // Handle share password requirement
          this.attemptedPasswordLogin = this.sharePassword !== "";
          // Reset password validation state on wrong password
          mutations.setShareData({ passwordValid: false });
          this.showPasswordPrompt();
        } else {
          router.push({ name: "error" });
        }
      } finally {
        mutations.setLoading(getters.isShare() ? "share" : "files", false);
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

    async fetchShareData() {
      // Parse share route
      let urlPath = getters.routePath('public/share')
      let parts = urlPath.split("/");
      this.shareHash = parts[1]
      this.shareSubPath = "/" + parts.slice(2).join("/");

      // Handle password
      if (this.sharePassword === "") {
        this.sharePassword = localStorage.getItem("sharepass:" + this.shareHash);
      } else {
        localStorage.setItem("sharepass:" + this.shareHash, this.sharePassword);
      }
      if (this.sharePassword === null) {
        this.sharePassword = "";
      }

      mutations.resetSelected();
      mutations.setMultiple(false);

      if (shareInfo.singleFileShare) {
        mutations.setSidebarVisible(true);
      }
      // Initialize password validation state for password-protected shares
      if (shareInfo.isPasswordProtected) {
        mutations.setShareData({ passwordValid: false });
      }
      // Fetch share data
      let file = await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword);
      file.hash = this.shareHash;
      this.shareToken = file.token;
      // Store share data in state for use by components
      mutations.setShareData({
        hash: this.shareHash,
        token: this.shareToken,
        subPath: this.shareSubPath,
        passwordValid: true,
      });

      // If not a directory, fetch content AND parent directory in parallel
      if (file.type != "directory") {
        const content = !getters.fileViewingDisabled(file.name);
        let directoryPath = url.removeLastDir(this.shareSubPath);
        // If directoryPath is empty, the file is in root - use '/' as the directory
        if (!directoryPath || directoryPath === '') {
          directoryPath = '/';
        }
        // Fetch parent directory unless it's the same as the file path
        const shouldFetchParent = directoryPath !== this.shareSubPath;
        // Run both fetches in parallel to minimize total API calls
        const promises = [
          publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, content)
        ];
          if (shouldFetchParent) {
            promises.push(
              publicApi.fetchPub(directoryPath, this.shareHash, this.sharePassword, false).catch(() => null)
            );
          }

        const results = await Promise.all(promises);
        file = results[0];
        file.hash = this.shareHash;
        this.shareToken = results[0].token;

        // Store the parent directory items for Preview to use
        if (shouldFetchParent && results[1] && results[1].items) {
          file.parentDirItems = results[1].items;
        }
      }

      mutations.replaceRequest(file);
      document.title = `${document.title} - ${file.name}`;
    },

    async fetchFilesData() {

      if (!getters.isLoggedIn()) {
        return;
      }

      // Clear share data when accessing files
      mutations.clearShareData();

      const routePath = url.removeTrailingSlash(getters.routePath(`${globalVars.baseURL}files`));
      const rootRoute =
        routePath == "/files" ||
        routePath == "/files/" ||
        routePath == "" ||
        routePath == "/";

      // lets redirect if multiple sources and user went to /files/
      if (state.serverHasMultipleSources && rootRoute) {
        const targetPath = `/files/${state.sources.current}`;
        // Prevent infinite loop by checking if we're already at the target path
        if (routePath !== targetPath) {
          router.push(targetPath);
          return;
        }
      }

      const result = extractSourceFromPath(getters.routePath());

      if (result.source === "") {
        // No sources available - show a more graceful message instead of error popup
        this.error = { message: $t("index.noSources") };
        mutations.replaceRequest({});
        return;
      }

      this.lastHash = "";
      // Reset view information using mutations
      mutations.resetSelected();
      let data = {};
      try {
        const fetchSource = decodeURIComponent(result.source);
        const fetchPath = decodeURIComponent(result.path);
        // Fetch initial data
        let res = await filesApi.fetchFiles(fetchSource, fetchPath );

        // If not a directory, fetch content AND parent directory in parallel
        if (res.type != "directory" && !res.type.startsWith("image")) {
          const content = !getters.fileViewingDisabled(res.name);
          let directoryPath = url.removeLastDir(res.path);

          // If directoryPath is empty, the file is in root - use '/' as the directory
          if (!directoryPath || directoryPath === '') {
            directoryPath = '/';
          }

          // Fetch parent directory unless it's the same as the file path
          const shouldFetchParent = directoryPath !== res.path;

          // Run both fetches in parallel to minimize total API calls
          const promises = [
            filesApi.fetchFiles(res.source, res.path, content)
          ];

          if (shouldFetchParent) {
            promises.push(
              filesApi.fetchFiles(res.source, directoryPath).catch(() => null)
            );
          }

          const results = await Promise.all(promises);
          res = results[0];

          // Store the parent directory items for Preview to use
          if (shouldFetchParent && results[1] && results[1].items) {
            res.parentDirItems = results[1].items;
          }
        }
        data = res;

        if (state.sources.count > 1) {
          mutations.setCurrentSource(data.source);
        }
        document.title = `${document.title} - ${res.name}`;
      } catch (e) {
        this.error = e;
        mutations.replaceRequest({});
      } finally {
        mutations.replaceRequest(data);
        mutations.setLoading("files", false);
      }
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
      if (event.keyCode === 112) {
        event.preventDefault();
        mutations.showHover("help"); // Use mutation
      }

      // Ctrl+, - navigate to settings
      if (event.ctrlKey && event.key === ',') {
        event.preventDefault();
        router.push('/settings');
      }

      // Esc! - for shares, reset selection
      if ( getters.isShare() && event.keyCode === 27) {
        if (getters.selectedCount() > 0) {
          mutations.resetSelected();
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
</style>
