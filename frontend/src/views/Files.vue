<template>
  <div>
    <breadcrumbs v-if="showBreadCrumbs" :base="isShare ? `/share/${shareHash}` : undefined" />
    <!-- Share password prompt -->
    <div v-if="isShare && error && error.status === 401" class="card floating" id="password">
      <div v-if="attemptedPasswordLogin" class="share__wrong__password">
        {{ $t("login.wrongCredentials") }}
      </div>
      <div class="card-title">
        <h2>{{ $t("general.password") }}</h2>
      </div>
      <div class="card-content">
        <input
          v-focus
          type="password"
          :placeholder="$t('general.password')"
          v-model="sharePassword"
          @keyup.enter="fetchData"
        />
      </div>
      <div class="card-action">
        <button
          class="button button--flat"
          @click="fetchData"
          :aria-label="$t('buttons.submit')"
          :title="$t('buttons.submit')"
        >
          {{ $t("buttons.submit") }}
        </button>
      </div>
    </div>
    <errors v-else-if="error" :errorCode="error.status" />
    <component v-else-if="currentViewLoaded" :is="currentView"></component>
    <div v-else>
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ $t("files.loading") }}</span>
      </h2>
    </div>
  </div>
  <PopupPreview v-if="popupEnabled" />
</template>

<script>
import { filesApi, publicApi } from "@/api";
import { notify } from "@/notify";
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
import { baseURL } from "@/utils/constants";
import PopupPreview from "@/components/files/PopupPreview.vue";
import { extractSourceFromPath } from "@/utils/url";

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
    isShare() {
      return getters.isShare();
    },
    popupEnabled() {
      return state.user.preview.popup;
    },
    showBreadCrumbs() {
      return getters.showBreadCrumbs();
    },
    currentView() {
      return getters.currentView();
    },
    currentViewLoaded() {
      return getters.currentView() !== null;
    },
    reload() {
      return state.reload;
    },
  },
  created() {
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
      if (window.location.hash === this.lastHash) return;
      this.lastHash = window.location.hash;
      if (window.location.hash) {
        const id = url.base64Encode(window.location.hash.slice(1));
        const element = document.getElementById(id);
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
      }
    },
    async fetchData() {
      if (state.deletedItem) {
        return
      }

      if (!state.user.sorting) {
        mutations.updateListingSortConfig({
          field: "name",
          asc: true,
        });
      }
      // Set loading and reset error
      mutations.setLoading(this.isShare ? "share" : "files", true);
      this.error = null;
      mutations.setReload(false);

      try {
        if (this.isShare) {
          await this.fetchShareData();
          console.log('Share data after fetch:')
        } else {
          await this.fetchFilesData();
        }
      } catch (e) {
        notify.showError(e.message);
        this.error = e;
        mutations.replaceRequest({});
        if (e.status === 404) {
          router.push({ name: "notFound" });
        } else if (e.status === 403) {
          router.push({ name: "forbidden" });
        } else if (e.status === 401 && this.isShare) {
          // Handle share password requirement
          this.attemptedPasswordLogin = this.sharePassword !== "";
        } else {
          router.push({ name: "error" });
        }
      } finally {
        mutations.setLoading(this.isShare ? "share" : "files", false);
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

      console.log('Share data:', {
        hash: this.shareHash,
        subPath: this.shareSubPath,
        password: this.sharePassword
      });

      // Store share data in state for use by components
      mutations.setShareData({
        hash: this.shareHash,
        token: this.shareToken,
        subPath: this.shareSubPath,
      });

      // Handle password
      if (this.sharePassword === "" || this.sharePassword === null) {
        this.sharePassword = localStorage.getItem("sharepass:" + this.shareHash);
      } else {
        localStorage.setItem("sharepass:" + this.shareHash, this.sharePassword);
      }
      // Get public user if not logged in
      if (!getters.isLoggedIn()) {
        mutations.setCurrentUser(getters.publicUser());
      }

      mutations.resetSelected();
      mutations.setMultiple(false);
      mutations.closeHovers();

      // Fetch share data
      let file = await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword);
      file.hash = this.shareHash;
      this.shareToken = file.token;

      // If not a directory, fetch content for preview components
      if (file.type != "directory") {
        const content = !getters.fileViewingDisabled(file.name);
        console.log('Share file content fetch debug:', {
          name: file.name,
          type: file.type,
          fileViewingDisabled: getters.fileViewingDisabled(file.name),
          content: content,
          disableViewingExt: state.user.disableViewingExt,
          disableOfficePreviewExt: state.user.disableOfficePreviewExt
        });

        file = await publicApi.fetchPub(this.shareSubPath, this.shareHash, this.sharePassword, content);
        file.hash = this.shareHash;
        this.shareToken = file.token;

        console.log('Share file after content fetch:', {
          hasContent: 'content' in file,
          contentLength: file.content ? file.content.length : 0,
          contentPreview: file.content ? file.content.substring(0, 50) + '...' : null
        });
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

      const routePath = url.removeTrailingSlash(getters.routePath(`${baseURL}files`));
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
        notify.showError($t("index.noSources"));
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
        // If not a directory, fetch content
        if (res.type != "directory" && !res.type.startsWith("image")) {
          const content = !getters.fileViewingDisabled(res.name);
          res = await filesApi.fetchFiles(res.source, res.path, content);
        }
        data = res;
        if (state.sources.count > 1) {
          mutations.setCurrentSource(data.source);
        }
        document.title = `${document.title} - ${res.name}`;
      } catch (e) {
        notify.showError(e);
        this.error = e;
        mutations.replaceRequest({});
      } finally {
        mutations.replaceRequest(data);
        mutations.setLoading("files", false);
      }
    },
    keyEvent(event) {
      // F1!
      if (event.keyCode === 112) {
        event.preventDefault();
        mutations.showHover("help"); // Use mutation
      }

      // Esc! - for shares, reset selection
      if (this.isShare && event.keyCode === 27) {
        if (getters.selectedCount() > 0) {
          mutations.resetSelected();
        }
      }
    },
  },
};
</script>

<style scoped>
.share__wrong__password {
  color: #ff4757;
  text-align: center;
  padding: 1em 0;
}

#password {
  max-width: 400px;
  margin: 2em auto;
}

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
</style>
