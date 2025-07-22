<template>
  <div class="share-sidebar">
    <div class="share-sidebar__header">
      <h3>
        {{
          req.type == "directory"
            ? $t("download.downloadFolder")
            : $t("download.downloadFile")
        }}
      </h3>
    </div>

    <div class="share-sidebar__content">
      <!-- File Preview -->
      <div class="share-sidebar__preview">
        <div
          v-if="isImage"
          class="share-sidebar__image"
        >
          <img :src="getLink(true)" alt="Preview" />
        </div>
        <div
          v-else-if="isMedia"
          class="share-sidebar__media"
        >
          <video width="100%" controls>
            <source :src="getLink(true)" type="video/mp4" />
          </video>
        </div>
        <div v-else class="share-sidebar__icon">
          <i class="material-icons">{{ icon }}</i>
        </div>
      </div>

      <!-- File Information -->
      <div class="share-sidebar__info">
        <div class="share-sidebar__info-item">
          <label>{{ $t("prompts.displayName") }}</label>
          <span>{{ req.name }}</span>
        </div>

        <div class="share-sidebar__info-item" :title="modTime">
          <label>{{ $t("prompts.lastModified") }}</label>
          <span>{{ humanTime }}</span>
        </div>

        <div class="share-sidebar__info-item">
          <label>{{ $t("prompts.size") }}</label>
          <span>{{ humanSize }}</span>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="share-sidebar__actions">
        <a
          target="_blank"
          :href="getLink(false)"
          class="button button--primary button--block"
        >
          <i class="material-icons">file_download</i>
          {{ $t("buttons.download") }}
        </a>

        <a
          v-if="req.type != 'directory'"
          target="_blank"
          :href="getLink(true)"
          class="button button--secondary button--block"
        >
          <i class="material-icons">open_in_new</i>
          {{ $t("buttons.openFile") }}
        </a>
      </div>

      <!-- QR Code -->
      <div class="share-sidebar__qr">
        <div class="share-sidebar__qr-code">
          <qrcode-vue :value="getLink(false)" size="150" level="M"></qrcode-vue>
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
import QrcodeVue from "qrcode.vue";

export default {
  name: "SidebarShare",
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
        return state.req.items ? state.req.items.length + " items" : "0 items";
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
.share-sidebar {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.share-sidebar__header {
  padding: 1rem;
  border-bottom: 1px solid var(--divider);
  background: var(--surfaceSecondary);
}

.share-sidebar__header h3 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--textPrimary);
}

.share-sidebar__content {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
}

.share-sidebar__preview {
  margin-bottom: 1.5rem;
  text-align: center;
}

.share-sidebar__image img {
  max-width: 100%;
  max-height: 200px;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.share-sidebar__media video {
  max-height: 200px;
  border-radius: 8px;
}

.share-sidebar__icon {
  color: var(--textSecondary);
  font-size: 4rem;
  margin: 1rem 0;
}

.share-sidebar__icon i {
  font-size: 4rem;
}

.share-sidebar__info {
  margin-bottom: 1.5rem;
}

.share-sidebar__info-item {
  display: flex;
  flex-direction: column;
  margin-bottom: 0.75rem;
  gap: 0.25rem;
}

.share-sidebar__info-item label {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--textSecondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.share-sidebar__info-item span {
  color: var(--textPrimary);
  word-break: break-all;
}

.share-sidebar__actions {
  margin-bottom: 1.5rem;
}

.share-sidebar__actions .button {
  width: 100%;
  margin-bottom: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.75rem;
  text-decoration: none;
  border-radius: 6px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.button--primary {
  background: var(--primary);
  color: white;
}

.button--primary:hover {
  background: var(--primaryHover);
}

.button--secondary {
  background: var(--surfaceSecondary);
  color: var(--textPrimary);
  border: 1px solid var(--divider);
}

.button--secondary:hover {
  background: var(--surfaceTertiary);
}

.share-sidebar__qr {
  text-align: center;
}

.share-sidebar__qr label {
  display: block;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--textSecondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 0.75rem;
}

.share-sidebar__qr-code {
  display: inline-block;
  padding: 0.5rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* Dark mode adjustments */
.dark-mode .share-sidebar__qr-code {
  background: var(--surfacePrimary);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .share-sidebar__content {
    padding: 0.75rem;
  }

  .share-sidebar__header {
    padding: 0.75rem;
  }

  .share-sidebar__icon {
    font-size: 3rem;
  }

  .share-sidebar__icon i {
    font-size: 3rem;
  }
}
</style>
