<template>
  <div class="card-title">
    <h2>{{ $t("share.shareInfo") }}</h2>
  </div>

  <div class="card-content">
    <div class="share-info-content">
      <div v-if="shareInfo.banner" class="banner">
        <img :src="getShareBanner" />
      </div>
      <div v-if="shareInfo.title" class="share-info-element">
        <h3>{{ shareInfo.title }}</h3>
      </div>
      <div v-if="shareInfo.description" class="share-info-element">
        <p>{{ shareInfo.description }}</p>
      </div>

      <div v-if="showShareInfo">
        <hr v-if="shareInfo.banner || shareInfo.title || shareInfo.description" />
        <div class="share-info-element">
          <strong>{{ $t("prompts.displayName") }}</strong> {{ req.name }}
        </div>
        <div class="share-info-element" :title="modTime">
          <strong>{{ $t("prompts.lastModified", { suffix: ":" }) }}</strong> {{ humanTime }}
        </div>
        <div class="share-info-element">
          <strong>{{ $t("prompts.size", { suffix: ":" }) }}</strong> {{ humanSize }}
        </div>
      </div>

      <div v-if="req.type" class="share-info-element share-info-center">
        <qrcode-vue class="qrcode" :value="getShareLink()" size="200" level="M"></qrcode-vue>
        <p class="share-link-text">{{ getShareLink() }}</p>
      </div>
    </div>
  </div>

  <div class="card-action">
    <button
      class="button button--flat button--grey"
      @click="close"
      :aria-label="$t('general.close')"
      :title="$t('general.close')"
    >
      {{ $t("general.close") }}
    </button>
  </div>
</template>

<script>
import { publicApi } from "@/api";
import { state, getters, mutations } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import QrcodeVue from "qrcode.vue";

export default {
  name: "ShareInfo",
  components: {
    QrcodeVue,
  },
  computed: {
    showShareInfo() {
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
      if (!state.req.modified) return "";
      if (state.req.type == "directory") {
        return state.req.items.length + " items (" + getHumanReadableFilesize(state.req.size) + ")";
      }
      return getHumanReadableFilesize(state.req.size);
    },
    humanTime() {
      if (!state.req.modified) return "";
      return getters.getTime(state.req.modified);
    },
    modTime() {
      return new Date(Date.parse(state.req.modified)).toLocaleString();
    },
  },
  methods: {
    getShareLink() {
      return state.shareInfo.shareURL;
    },
    close() {
      mutations.closeHovers();
    },
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

.share-info-content {
  display: flex;
  flex-direction: column;
}

.share-info-element {
  margin: 0.5em 0;
}

.share-info-center {
  text-align: center;
}

.share-info-center .qrcode {
  margin: 1em auto;
  display: block;
}

.share-link-text {
  margin-top: 1em;
  word-break: break-all;
  font-size: 0.9em;
  color: var(--textSecondary);
}

.share-info-element canvas {
  border-style: solid;
}
</style>

