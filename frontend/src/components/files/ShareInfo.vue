<template>
  <div class="share">
    <div class="share__box share__box__info">
      <div class="share__box__header">
        {{
          req.type == "directory"
            ? $t("download.downloadFolder")
            : $t("download.downloadFile")
        }}
      </div>

      <div v-if="isImage" class="share__box__element share__box__center share__box__icon">
        <img :src="getLink(true)" width="500px" />
      </div>
      <div v-else-if="isMedia" class="share__box__element share__box__center share__box__icon">
        <video width="500" height="500" controls>
          <source :src="getLink(true)" type="video/mp4" />
        </video>
      </div>
      <div v-else class="share__box__element share__box__center share__box__icon">
        <i class="material-icons">{{ icon }}</i>
      </div>

      <div class="share__box__element">
        <strong>{{ $t("prompts.displayName") }}</strong> {{ req.name }}
      </div>
      <div class="share__box__element" :title="modTime">
        <strong>{{ $t("prompts.lastModified") }}:</strong> {{ humanTime }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </div>
      <div class="share__box__element">
        <strong>{{ $t("prompts.size") }}:</strong> {{ humanSize }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </div>

      <div class="share__box__element share__box__center">
        <a target="_blank" :href="getLink(false)" class="button button--flat">
          <div>
            <i class="material-icons">file_download</i>{{ $t("buttons.download") }}
          </div>
        </a>
        <a target="_blank" :href="getLink(true)" class="button button--flat" v-if="req.type != 'directory'">
          <div>
            <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
          </div>
        </a>
      </div>

      <div class="share__box__element share__box__center">
        <qrcode-vue :value="getLink(false)" size="200" level="M"></qrcode-vue>
      </div>
    </div>
  </div>
</template>

<script>
import { publicApi } from "@/api";
import { state, getters } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { getTypeInfo } from "@/utils/mimetype";
import QrcodeVue from "qrcode.vue";

export default {
  name: "ShareInfo",
  components: {
    QrcodeVue,
  },
  props: {
    hash: {
      type: String,
      required: true,
    },
    token: {
      type: String,
      required: true,
    },
    subPath: {
      type: String,
      default: "/",
    },
  },
  computed: {
    req() {
      return state.req;
    },
    icon() {
      if (state.req.type == "directory") return "folder";
      if (getTypeInfo(state.req.type).simpleType == "image") return "insert_photo";
      if (getTypeInfo(state.req.type).simpleType == "audio") return "volume_up";
      if (getTypeInfo(state.req.type).simpleType == "video") return "movie";
      return "insert_drive_file";
    },
    humanSize() {
      if (state.req.type == "directory") {
        return state.req.items ? state.req.items.length : 0;
      }
      return getHumanReadableFilesize(state.req.size);
    },
    humanTime() {
      return getters.getTime(state.req.modified);
    },
    modTime() {
      return new Date(Date.parse(state.req.modified)).toLocaleString();
    },
    isImage() {
      return state.req.type ? getTypeInfo(state.req.type).simpleType === "image" : false;
    },
    isMedia() {
      if (!state.req.type) return false;
      return (
        getTypeInfo(state.req.type).simpleType === "video" ||
        getTypeInfo(state.req.type).simpleType === "audio"
      );
    },
  },
  methods: {
    getLink(inline = false) {
      return publicApi.getDownloadURL({
        path: this.subPath,
        hash: this.hash,
        token: this.token,
        inline: inline,
      });
    },
  },
};
</script>

<style scoped>
.share {
  margin: 1em 0;
}

.share__box {
  background: white;
  border-radius: 0.3em;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.12), 0 1px 2px rgba(0, 0, 0, 0.24);
  margin: 1em;
  padding: 1em;
}

.share__box__header {
  font-size: 1.2em;
  font-weight: bold;
  text-align: center;
  padding-bottom: 1em;
  border-bottom: 1px solid #eee;
  margin-bottom: 1em;
}

.share__box__element {
  margin: 0.5em 0;
}

.share__box__center {
  text-align: center;
}

.share__box__icon {
  font-size: 4em;
  color: #6c7b7f;
}

.share__box__icon i {
  font-size: 4em;
}

.share__box__icon img,
.share__box__icon video {
  max-width: 100%;
  height: auto;
  border-radius: 0.3em;
}

.button {
  display: inline-flex;
  align-items: center;
  gap: 0.5em;
  margin: 0.25em;
}

.button i {
  font-size: 1.2em;
}

/* Dark mode support */
.dark-mode .share__box {
  background: var(--surfacePrimary);
  color: var(--textPrimary);
}

.dark-mode .share__box__header {
  border-color: var(--divider);
}
</style>