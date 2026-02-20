<template>
  <div class="card-content">
    <div v-show="creating" class="loading-content">
      <LoadingSpinner size="small" mode="placeholder" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!creating">
      <template v-if="showFileList">
        <file-list
          ref="fileList"
          :browse-path="currentPath"
          :show-folders="true"
          :show-files="false"
          @update:selected="updateDestination"
        />
      </template>
      <template v-else>
        <p>{{ $t("prompts.archiveMessage") }}</p>
        <p class="prompts-label">{{ $t("prompts.archiveDestination") }}</p>
        <div
          aria-label="archive-destination"
          class="searchContext clickable button"
          @click="showFileList = true"
        >
          {{ $t("general.path", { suffix: ":" }) }} {{ destPath }}{{ destSource ? ` (${destSource})` : "" }}
        </div>
        <p class="prompts-label">{{ $t("prompts.archiveName") }}</p>
        <input
          v-model.trim="archiveName"
          class="input"
          type="text"
          :placeholder="defaultArchiveName"
        />
        <p class="prompts-label">{{ $t("general.format", { suffix: ":" }) }}</p>
        <select v-model="format" class="input">
          <option value="zip">zip</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <option value="tar.gz">tar.gz</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </select>
        <p v-if="format === 'tar.gz'" class="prompts-label">{{ $t("prompts.archiveCompression") }}</p>
        <input
          v-if="format === 'tar.gz'"
          v-model.number="compression"
          class="input"
          type="number"
          min="0"
          max="9"
        />
        <div class="archive-options settings-items">
          <ToggleSwitch
            class="item"
            v-model="deleteAfter"
            :name="$t('prompts.archiveDeleteAfter')"
          />
        </div>
      </template>
    </div>
  </div>
  <div class="card-actions">
    <template v-if="showFileList">
      <button
        class="button button--flat button--grey"
        @click="showFileList = false"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        class="button button--flat"
        @click="showFileList = false"
        :aria-label="$t('general.select', { suffix: '' })"
        :title="$t('general.select', { suffix: '' })"
      >
        {{ $t("general.select", { suffix: "" }) }}
      </button>
    </template>
    <template v-else>
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        class="button button--flat"
        :disabled="!canSubmit || creating"
        :aria-label="$t('prompts.archive')"
        :title="$t('prompts.archive')"
        @click="submit"
      >
        {{ $t("prompts.archive") }}
      </button>
    </template>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { notify } from "@/notify";
import { archiveApi } from "@/api";
import { goToItem } from "@/utils/url";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import FileList from "@/components/files/FileList.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "archive",
  components: { LoadingSpinner, FileList, ToggleSwitch },
  props: {
    items: {
      type: Array,
      required: true,
    },
    source: {
      type: String,
      required: true,
    },
    currentPath: {
      type: String,
      default: "/",
    },
  },
  data() {
    return {
      destPath: "/",
      destSource: null,
      archiveName: "",
      format: "zip",
      compression: 0,
      creating: false,
      showFileList: false,
      deleteAfter: true,
    };
  },
  mounted() {
    if (this.currentPath) {
      this.destPath = this.currentPath.replace(/\/+$/, "") || "/";
      if (!this.destPath.startsWith("/")) this.destPath = "/" + this.destPath;
    }
  },
  computed: {
    defaultArchiveName() {
      return this.format === "tar.gz" ? "archive.tar.gz" : "archive.zip";
    },
    canSubmit() {
      const name = this.archiveName || this.defaultArchiveName;
      return name.length > 0 && this.destPath;
    },
    fullDestination() {
      const name = this.archiveName || this.defaultArchiveName;
      const base = (this.destPath || "/").replace(/\/+$/, "");
      const path = base === "" ? "/" + name : base + "/" + name;
      return path.replace(/\/+/g, "/");
    },
  },
  methods: {
    closeHovers() {
      mutations.closeTopHover();
    },
    updateDestination(pathOrData) {
      if (typeof pathOrData === "string") {
        this.destPath = pathOrData;
      } else if (pathOrData && pathOrData.path) {
        this.destPath = pathOrData.path;
        this.destSource = pathOrData.source;
      }
    },
    async submit() {
      if (!this.canSubmit) return;
      this.creating = true;
      try {
        let dest = this.fullDestination;
        const ext = this.format === "tar.gz" ? ".tar.gz" : ".zip";
        if (!dest.endsWith(".zip") && !dest.endsWith(".tar.gz") && !dest.endsWith(".tgz")) {
          dest = dest + ext;
        }
        const payload = {
          fromSource: this.source,
          paths: this.items.map((it) => (typeof it === "string" ? it : it.path)),
          destination: dest,
          format: this.format,
          compression: this.compression,
        };
        if (this.destSource && this.destSource !== this.source) {
          payload.toSource = this.destSource;
        }
        if (this.deleteAfter) {
          payload.deleteAfter = true;
        }
        await archiveApi.createArchive(payload);
        mutations.setReload(true);
        mutations.closeTopHover();

        const destSource = this.destSource || this.source;
        const archivePath = dest;
        const buttonAction = () => {
          if (archivePath) {
            goToItem(destSource || null, archivePath, {});
          }
        };
        notify.showSuccess(this.$t("prompts.archiveSuccess"), {
          icon: "folder",
          buttons: archivePath ? [{ label: this.$t("buttons.goToItem"), primary: true, action: buttonAction }] : undefined,
        });
      } catch (err) {
        console.error(err);
      } finally {
        this.creating = false;
      }
    },
  },
};
</script>

<style scoped>
.loading-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding-top: 2em;
  min-height: 200px;
}
.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}
.prompts-label {
  margin-top: 1em;
  margin-bottom: 0.25em;
  font-weight: 500;
}
.archive-options {
  margin-top: 1em;
}
.card-content {
  position: relative;
}
</style>
