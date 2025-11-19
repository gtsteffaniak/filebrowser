<template>
  <div class="share card">
    <div class="share__box">
      <div v-if="shareInfo.banner" class="banner">
        <img :src="getShareBanner" />
      </div>
      <div v-if="shareInfo.title" class="share__box__element">
        <h3>{{ shareInfo.title }}</h3>
      </div>
      <div v-if="shareInfo.description" class="share__box__element">
        <p>{{ shareInfo.description }}</p>
      </div>

      <div>
        <hr v-if="shareInfo.banner || shareInfo.title || shareInfo.description" />
        <div v-if="showShareInfo">
          <div class="share__box__element">
            <strong>{{ $t("prompts.displayName") }}</strong> {{ req.name }}
          </div>
          <div class="share__box__element" :title="modTime">
            <strong>{{ $t("prompts.lastModified", { suffix: ":" }) }}</strong> {{ humanTime }}
            <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          </div>
          <div class="share__box__element">
            <strong>{{ $t("prompts.size", { suffix: ":" }) }}</strong> {{ humanSize }}
            <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          </div>
          <div v-if="!shareInfo.disableDownload" class="share__box__element share__box__center">
            <button class="button button--flat clickable" @click="goToDownload()"> {{ $t("buttons.download") }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { publicApi } from "@/api";
import { state, getters } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { getTypeInfo } from "@/utils/mimetype";

export default {
  name: "ShareInfo",
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
    showShareInfo() {
      // Don't show file/folder info if req is not loaded or empty
      if (!state.req || !state.req.name) {
        return false;
      }
      if (state.shareInfo?.shareType !== 'normal') {
        return false;
      }
      if (!state.shareInfo?.isPasswordProtected) {
        return true
      }
      return state.share.passwordValid
    },
    getShareBanner() {
      if (state.shareInfo?.banner.startsWith("http")) {
        return state.shareInfo?.banner;
      }
      return publicApi.getDownloadURL(state.share, [state.shareInfo?.banner]);
    },
    shareInfo() {
      return state.shareInfo;
    },
    req() {
      return state.req;
    },
    humanSize() {
      if (!state.req || !state.req.modified) return "";
      if (state.req.type == "directory") {
        return state.req.items.length + " items (" + getHumanReadableFilesize(state.req.size) + ")";
      }
      return getHumanReadableFilesize(state.req.size);
    },
    humanTime() {
      if (!state.req || !state.req.modified) return "";
      return getters.getTime(state.req.modified);
    },
    modTime() {
      if (!state.req || !state.req.modified) return "";
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
  },
};
</script>

<style scoped>
.banner {
  width: 100%;
  padding-bottom: 1em;
}

.banner img {
  width: 100%;
}

.share {
  display: flex;
}

.share__box {
  width: 100%;
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

.share__box__element canvas {
  border-style: solid;
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
</style>