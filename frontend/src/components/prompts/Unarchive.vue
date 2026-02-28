<template>
  <div class="card-content">
    <div v-show="isLoading" class="loading-content">
      <LoadingSpinner size="small" mode="placeholder" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!isLoading">
      <template v-if="showFileList">
        <file-list
          ref="fileList"
          :browse-path="parentPath"
          :browse-source="itemSource"
          :show-folders="true"
          :show-files="false"
          @update:selected="updateDestination"
        />
      </template>
      <template v-else>
        <p>{{ $t("prompts.unarchiveMessage") }}</p>
        <p class="prompts-label">{{ $t("prompts.unarchiveDestination") }}</p>
        <div
          aria-label="unarchive-destination"
          class="searchContext clickable button"
          @click="showFileList = true"
        >
          {{ $t("general.path", { suffix: ":" }) }} {{ destPath }}{{ destSource ? ` (${destSource})` : "" }}
        </div>
        <div class="unarchive-options settings-items">
          <ToggleSwitch
            class="item"
            v-model="deleteAfter"
            :name="$t('prompts.unarchiveDeleteAfter')"
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
        :disabled="!destPath || !isDirSelection || isLoading"
        :aria-label="$t('prompts.unarchive')"
        :title="$t('prompts.unarchive')"
        @click="submit"
      >
        {{ $t("prompts.unarchive") }}
      </button>
    </template>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { url } from "@/utils";
import { notify } from "@/notify";
import { resourcesApi } from "@/api";
import { goToItem } from "@/utils/url";
import FileList from "@/components/files/FileList.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "unarchive",
  components: { FileList, LoadingSpinner, ToggleSwitch },
  props: {
    item: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      destPath: "/",
      destSource: null,
      destType: null,
      deleteAfter: true,
      isLoading: false,
      showFileList: false,
    };
  },
  mounted() {
    this.destPath = this.parentPath || "/";
    this.destSource = this.itemSource;
  },
  computed: {
    itemSource() {
      return this.item.source || this.item.fromSource;
    },
    itemPath() {
      return this.item.path || this.item.from;
    },
    parentPath() {
      if (!this.itemPath) return "/";
      return url.removeLastDir(this.itemPath) + "/";
    },
    isDirSelection() {
      return this.destType === "directory" || !this.destType;
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
        this.destType = pathOrData.type;
      }
    },
    async submit() {
      if (!this.destPath || !this.isDirSelection) return;
      this.isLoading = true;
      try {
        const toSource = this.destSource || this.itemSource;
        await resourcesApi.unarchive({
          fromSource: this.itemSource,
          toSource: toSource !== this.itemSource ? toSource : undefined,
          path: this.itemPath,
          destination: this.destPath,
          deleteAfter: this.deleteAfter,
        });
        mutations.setReload(true);
        mutations.closeTopHover();

        const destPath = this.destPath;
        const destSource = toSource;
        const buttonAction = () => destPath && goToItem(destSource, destPath, {});
        notify.showSuccess(this.$t("prompts.unarchiveSuccess"), {
          icon: "folder",
          buttons: destPath ? [{ label: this.$t("buttons.goToItem"), primary: true, action: buttonAction }] : undefined,
        });
      } catch (err) {
        console.error(err);
      } finally {
        this.isLoading = false;
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
.unarchive-options {
  margin-top: 1em;
}
.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5em;
  cursor: pointer;
}
.card-content {
  position: relative;
}
</style>
