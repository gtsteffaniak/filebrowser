<template>
  <div class="card-title">
    <h2>{{ $t("settings.systemAdmin") }}</h2>
  </div>
  <div class="card-content">
    <div class="settings-items">
      <ToggleSwitch class="item" v-model="localuser.disableUpdateNotifications" @change="updateSettings"
        :name="$t('profileSettings.disableUpdateNotifications')"
        :description="$t('profileSettings.disableUpdateNotificationsDescription')" />
      <!-- Config Viewer Section -->
      <div class="config-viewer-section">
        <h3>{{ $t('settings.configViewer') }}</h3>
        <div class="config-options">
          <label class="checkbox-label">
            <input type="checkbox" v-model="configOptions.showFull" @change="fetchConfig" />
          {{ $t('settings.configViewerShowFull') }}
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="configOptions.showComments" @change="fetchConfig" />
            {{ $t('settings.configViewerShowComments') }}
          </label>
        </div>
        <div>
          <button
           @click="fetchConfig" class="button"
           :disabled="configLoading"
           aria-label="loadConfig"
            style="margin-bottom: 1em;">
            {{ configLoading ? $t('files.loading') : $t('settings.configViewerLoadConfig') }}
          </button>
        </div>
        <div class="config-editor-container">
          <Editor :viewer-mode="true" :content="configContent" :editor-mode="'yaml'" :read-only="true" />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import * as settingsApi from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import Editor from "@/views/files/Editor.vue";

export default {
  name: "systemAdmin",
  components: {
    ToggleSwitch,
    Editor,
  },
  data() {
    return {
      localuser: { disableUpdateNotifications: false },
      configOptions: {
        showFull: false,
        showComments: false,
      },
      configContent: '',
      configLoading: false,
    };
  },
  computed: {},
  mounted() {
    // Initialize localuser with default values and merge with state.user
    this.localuser = {
      disableUpdateNotifications: false,
      ...state.user
    };
  },
  methods: {
    /**
     * @param {Event} event - The form event
     */
    async updateSettings(event) {
      if (event && typeof event.preventDefault === 'function') {
        event.preventDefault();
      }
      try {
        const data = this.localuser;
        mutations.updateCurrentUser(data);
        await usersApi.update(data, [
          "disableUpdateNotifications",
        ]);
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    async fetchConfig() {
      this.configLoading = true;
      try {
        const response = await settingsApi.getConfig(
          this.configOptions.showFull,
          this.configOptions.showComments
        );
        this.configContent = await response.text();
      } catch (e) {
        notify.showError(e);
        const errorMessage = (e && typeof e === 'object' && 'message' in e) ? String(e.message) : 'Unknown error';
        this.configContent = 'Error loading config: ' + errorMessage;
      } finally {
        this.configLoading = false;
      }
    },
  },
  };
</script>

<style scoped>
.card-content {
  margin-top: 1em;
}

.config-options {
  margin: 1em;
}

.checkbox-label {
  padding-right: 1em;
}

.config-editor-container {
  /* Make the container resizable */
  resize: vertical;
  width: 100%;
  resize: vertical;
  overflow: hidden;
  border: 1px solid #ccc;
  border-radius: 4px;
}

.config-editor-container :deep(.ace_editor) {
  min-height: 20em;
  width: 100%;
  height: 100%;
}

/* Ensure the editor has a minimum height when empty */
.config-editor-container :deep(#editor) {
  min-height: 20em;
}
</style>
