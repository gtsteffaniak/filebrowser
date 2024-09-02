<template>
  <div>
    <breadcrumbs :base="'/share/' + hash" />
    <div v-if="loading">
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ $t("files.loading") }}</span>
      </h2>
    </div>
    <div v-else-if="error">
      <div v-if="error.status === 401">
        <div class="card floating" id="password">
          <div v-if="attemptedPasswordLogin" class="share__wrong__password">
            {{ $t("login.wrongCredentials") }}
          </div>
          <div class="card-title">
            <h2>{{ $t("login.password") }}</h2>
          </div>

          <div class="card-content">
            <input
              v-focus
              type="password"
              :placeholder="$t('login.password')"
              v-model="password"
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
      </div>
      <errors v-else :errorCode="error.status" />
    </div>
    <div v-else>
      <div class="share">
        <div class="share__box share__box__info">
          <div class="share__box__header">
            {{ req.isDir ? $t("download.downloadFolder") : $t("download.downloadFile") }}
          </div>

          <div
            v-if="isImage"
            class="share__box__element share__box__center share__box__icon"
          >
            <img :src="inlineLink" width="500px" />
          </div>
          <div
            v-else-if="isMedia"
            class="share__box__element share__box__center share__box__icon"
          >
            <video width="500" height="500" controls>
              <source :src="inlineLink" type="video/mp4" />
            </video>
          </div>
          <div v-else class="share__box__element share__box__center share__box__icon">
            <i class="material-icons">{{ icon }}</i>
          </div>
          <div class="share__box__element">
            <strong>{{ $t("prompts.displayName") }}</strong> {{ req.name }}
          </div>
          <div class="share__box__element" :title="modTime">
            <strong>{{ $t("prompts.lastModified") }}:</strong> {{ humanTime }}
          </div>
          <div class="share__box__element">
            <strong>{{ $t("prompts.size") }}:</strong> {{ humanSize }}
          </div>
          <div class="share__box__element share__box__center">
            <a target="_blank" :href="link" class="button button--flat">
              <div>
                <i class="material-icons">file_download</i>{{ $t("buttons.download") }}
              </div>
            </a>
            <a
              target="_blank"
              :href="inlineLink"
              class="button button--flat"
              v-if="!req.isDir"
            >
              <div>
                <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
              </div>
            </a>
          </div>
          <div class="share__box__element share__box__center">
            <qrcode-vue :value="link" size="200" level="M"></qrcode-vue>
          </div>
        </div>
        <div
          v-if="req.isDir && req.items.length > 0"
          class="share__box share__box__items"
        >
          <div class="share__box__header" v-if="req.isDir">
            {{ $t("files.files") }}
          </div>
          <div id="listingView" class="list file-icons">
            <item
              v-for="item in req.items"
              :key="base64(item.name)"
              v-bind:index="item.index"
              v-bind:name="item.name"
              v-bind:isDir="item.isDir"
              v-bind:url="item.url"
              v-bind:modified="item.modified"
              v-bind:type="item.type"
              v-bind:size="item.size"
              readOnly
            >
            </item>

            <div :class="{ active: multiple }" id="multiple-selection">
              <p>{{ $t("files.multipleSelectionEnabled") }}</p>
              <div
                @click="setMultipleFalse"
                tabindex="0"
                role="button"
                :title="$t('files.clear')"
                :aria-label="$t('files.clear')"
                class="action"
              >
                <i class="material-icons">clear</i>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else-if="req.isDir && req.items.length === 0"
          class="share__box share__box__items"
        >
          <h2 class="message">
            <i class="material-icons">sentiment_dissatisfied</i>
            <span>{{ $t("files.lonely") }}</span>
          </h2>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { showSuccess } from "@/notify";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { pub as api } from "@/api";
import { fromNow } from "@/utils/moment";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import QrcodeVue from "qrcode.vue";
import Item from "@/components/files/ListingItem.vue";
import Clipboard from "clipboard";
import { state, getters, mutations } from "@/store";

export default {
  name: "share",
  components: {
    Breadcrumbs,
    Item,
    QrcodeVue,
    Errors,
  },
  data() {
    return {
      error: null,
      password: "",
      attemptedPasswordLogin: false,
      hash: null,
      token: null,
      clip: null,
    };
  },
  watch: {
    $route() {
      this.fetchData();
    },
  },
  created() {
    this.hash = state.route.params.path.at(0);
    this.fetchData();
  },
  mounted() {
    window.addEventListener("keydown", this.keyEvent);
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      showSuccess(this.$t("success.linkCopied"));
    });
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    this.clip.destroy();
  },
  computed: {
    setMultipleFalse() {
      return mutations.setMultiple(false);
    },
    req() {
      return state.req; // Access state directly from the store
    },
    loading() {
      return getters.isLoading(); // Access state directly from the store
    },
    multiple() {
      return state.multiple; // Access state directly from the store
    },
    selected() {
      return state.selected; // Access state directly from the store
    },
    selectedCount() {
      return getters.selectedCount(); // Access getter directly from the store
    },
    icon() {
      if (state.req.isDir) return "folder";
      if (state.req.type === "image") return "insert_photo";
      if (state.req.type === "audio") return "volume_up";
      if (state.req.type === "video") return "movie";
      return "insert_drive_file";
    },
    link() {
      console.log("pathing", window.location.pathname);
      return api.getDownloadURL({
        hash: this.hash,
        path: window.location.pathname,
      });
    },
    inlineLink() {
      return api.getDownloadURL(
        {
          hash: this.hash,
          path: window.location.pathname,
        },
        true
      );
    },
    humanSize() {
      if (state.req.isDir) {
        return state.req.items.length;
      }
      return getHumanReadableFilesize(state.req.size);
    },
    humanTime() {
      if (state.req.modified === undefined) return 0;
      return fromNow(state.req.modified, state.user.locale);
    },
    modTime() {
      return new Date(Date.parse(state.req.modified)).toLocaleString();
    },
    isImage() {
      return state.req.type === "image";
    },
    isMedia() {
      return state.req.type === "video" || state.req.type === "audio";
    },
  },
  methods: {
    base64(name) {
      return window.btoa(unescape(encodeURIComponent(name)));
    },
    async fetchData() {
      // Set loading to true and reset the error.
      mutations.setLoading("share", true);
      this.error = null;
      // Reset view information.
      if (!getters.isLoggedIn()) {
        let userData = await api.getPublicUser();
        mutations.setUser(userData);
      }
      mutations.setReload(false);
      mutations.resetSelected();
      mutations.setMultiple(false);
      mutations.closeHovers();

      let url = state.route.path;
      if (url === "") url = "/";
      if (url[0] !== "/") url = "/" + url;

      let file = await api.fetchPub(url, this.password);
      file.hash = this.hash;
      this.token = file.token || "";
      mutations.updateRequest(file);
      document.title = `${file.name} - ${document.title}`;
      mutations.setLoading("share", false);
    },
    keyEvent(event) {
      // Esc!
      if (event.keyCode === 27) {
        // If we're on a listing, unselect all files and folders.
        if (getters.selectedCount() > 0) {
          mutations.resetSelected();
        }
      }
    },
    toggleMultipleSelection() {
      mutations.setMultiple(!state.multiple);
    },
    download() {
      if (getters.isSingleFileSelected()) {
        api.download(null, this.hash, this.token, getters.selectedDownloadUrl());
        return;
      }

      mutations.showHover({
        name: "download",
        confirm: (format) => {
          mutations.closeHovers();

          let files = [];

          for (let i of this.selected) {
            files.push(state.req.items[i].path);
          }

          api.download(format, this.hash, this.token, ...files);
        },
      });
    },
    linkSelected() {
      return getters.isSingleFileSelected()
        ? api.getDownloadURL({
            hash: this.hash,
            path: state.req.items[this.selected[0]].path,
          })
        : "";
    },
  },
};
</script>
<style>
.share {
  padding-bottom: 35vh;
}
</style>
