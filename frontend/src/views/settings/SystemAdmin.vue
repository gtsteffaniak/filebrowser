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
      <div class="config-viewer-section settings-items">
        <h3>{{ $t('settings.configViewer') }}</h3>
        <ToggleSwitch
          class="item"
          v-model="configOptions.showFull"
          :name="$t('settings.configViewerShowFull')"
        />
        <ToggleSwitch
          class="item"
          v-model="configOptions.showComments"
          :name="$t('settings.configViewerShowComments')"
        />
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
  watch: {
    "configOptions.showFull"() {
      void this.fetchConfig();
    },
    "configOptions.showComments"() {
      void this.fetchConfig();
    },
  },
  mounted() {
    // Initialize localuser with default values and merge with state.user
    this.localuser = {
      disableUpdateNotifications: false,
      ...state.user
    };
    void this.fetchConfig();
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
        void mutations.updateCurrentUser(this.localuser);
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
      }
    },
    async fetchConfig() {
      this.configLoading = true;
      try {
        const response = await settingsApi.config(
          this.configOptions.showFull,
          this.configOptions.showComments
        );
        this.configContent = await response.text();
      } catch (e) {
        console.error(e);
        const errorMessage = (e && typeof e === 'object' && 'message' in e) ? String(e.message) : 'Unknown error';
        this.configContent = `Error loading config: ${errorMessage}`;
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

.config-viewer-section {
  margin-top: 1em;
}

.config-editor-container {
  /* Make the container resizable */
  resize: vertical;
  width: 100%;
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
