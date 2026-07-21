<template>
  <div class="card-content no-buttons prompt-panel user-defaults-prompt">
    <div class="user-defaults-scroll">
      <p v-if="lockedFromConfig" class="lock-message">{{ lockMessage }}</p>
      <div v-if="loading" class="loading-hint">{{ $t("general.loading") }}</div>

      <template v-else>
        <div :class="{ 'user-defaults-readonly': lockedFromConfig }">
          <UserProfilePreferences
            v-model="preferenceSections"
            enforceable
            :enforced="enforced"
            show-extension-inputs
            :show-thumbnail-master="false"
            @change="onPreferenceSectionChange"
            @enforced-change="onEnforcedChange"
          />
          <UserDefaultsAccountSection
            :account="values.account"
            :enforced="enforced.account || {}"
            :enforced-permissions="enforced.account?.permissions || {}"
            @account-change="onAccountFieldChange"
            @enforced-change="(field, value) => patchEnforcedFlag('account', field, value)"
            @enforced-permission-change="(field, value) => patchEnforcedFlag('account', `permissions.${field}`, value)"
          />
        </div>
      </template>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import * as settingsApi from "@/api/settings";
import { getObjectProperty } from "@/utils/object.js";
import UserProfilePreferences from "@/components/settings/UserProfilePreferences.vue";
import UserDefaultsAccountSection from "@/components/settings/UserDefaultsAccountSection.vue";

function emptyEnforced() {
  return {
    sidebar: {},
    listing: {},
    preview: {},
    fileViewer: {},
    search: {},
    ui: {},
    account: { permissions: {} },
    fileLoading: {},
  };
}

function emptyDefaults() {
  return {
    sidebar: {},
    listing: {},
    preview: {},
    fileViewer: {},
    search: {},
    ui: {},
    account: { permissions: {} },
    fileLoading: {},
  };
}

export default {
  name: "user-defaults",
  components: {
    UserProfilePreferences,
    UserDefaultsAccountSection,
  },
  data() {
    return {
      loading: true,
      saving: false,
      hydrating: false,
      lockedFromConfig: false,
      lockMessage: "",
      values: emptyDefaults(),
      enforced: emptyEnforced(),
    };
  },
  computed: {
    preferenceSections: {
      get() {
        const v = this.values;
        return {
          sidebar: { ...(v.sidebar || {}) },
          listing: { ...(v.listing || {}) },
          preview: { ...(v.preview || {}) },
          fileViewer: { ...(v.fileViewer || {}) },
          search: { ...(v.search || {}) },
          ui: { ...(v.ui || {}) },
          account: { ...(v.account || {}), permissions: { ...(v.account?.permissions || {}) } },
          fileLoading: { ...(v.fileLoading || {}) },
        };
      },
      set(sections) {
        this.values = {
          ...this.values,
          sidebar: { ...(sections.sidebar || {}) },
          listing: { ...(sections.listing || {}) },
          preview: { ...(sections.preview || {}) },
          fileViewer: { ...(sections.fileViewer || {}) },
          search: { ...(sections.search || {}) },
          ui: { ...(sections.ui || {}) },
          fileLoading: { ...(sections.fileLoading || {}) },
        };
      },
    },
  },
  mounted() {
    void this.load();
  },
  methods: {
    normalizePreviewBool(val, defaultValue = true) {
      if (val === undefined || val === null) {
        return defaultValue;
      }
      return !!val;
    },
    applyResponse(data) {
      this.hydrating = true;
      this.lockedFromConfig = !!data.lockedFromConfig;
      this.lockMessage =
        data.lockMessage || this.$t("settings.userDefaultsLockedFromConfig");
      const v = data.values || {};
      const enf = data.enforced || {};
      this.values = {
        ...emptyDefaults(),
        ...v,
        sidebar: { ...(v.sidebar || {}) },
        listing: { ...(v.listing || {}) },
        preview: {
          image: this.normalizePreviewBool(v.preview?.image),
          video: this.normalizePreviewBool(v.preview?.video),
          audio: this.normalizePreviewBool(v.preview?.audio),
          office: this.normalizePreviewBool(v.preview?.office),
          folder: this.normalizePreviewBool(v.preview?.folder),
          models: this.normalizePreviewBool(v.preview?.models),
          popup: this.normalizePreviewBool(v.preview?.popup),
          motionVideoPreview: this.normalizePreviewBool(v.preview?.motionVideoPreview),
          disablePreviewExt: v.preview?.disablePreviewExt || "",
        },
        fileViewer: { ...(v.fileViewer || {}) },
        search: { ...(v.search || {}) },
        ui: { ...(v.ui || {}) },
        account: {
          ...(v.account || {}),
          permissions: { ...(v.account?.permissions || {}) },
        },
        fileLoading: { ...(v.fileLoading || {}) },
      };
      this.enforced = {
        ...emptyEnforced(),
        sidebar: { ...(enf.sidebar || {}) },
        listing: { ...(enf.listing || {}) },
        preview: { ...(enf.preview || {}) },
        fileViewer: { ...(enf.fileViewer || {}) },
        search: { ...(enf.search || {}) },
        ui: { ...(enf.ui || {}) },
        account: {
          ...(enf.account || {}),
          permissions: { ...(enf.account?.permissions || {}) },
        },
        fileLoading: { ...(enf.fileLoading || {}) },
      };
      this.$nextTick(() => {
        this.hydrating = false;
      });
    },
    canPatch() {
      return !this.loading && !this.saving && !this.hydrating && !this.lockedFromConfig;
    },
    async load() {
      this.loading = true;
      this.hydrating = true;
      try {
        const data = await settingsApi.getUserDefaults();
        this.applyResponse(data);
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
      } finally {
        this.loading = false;
        this.$nextTick(() => {
          this.hydrating = false;
        });
      }
    },
    async sendPatch(partial) {
      if (!this.canPatch()) {
        return;
      }
      this.saving = true;
      try {
        await settingsApi.patchUserDefaults(partial);
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
        await this.load();
      } finally {
        this.saving = false;
      }
    },
    patchEnforcedFlag(section, key, value) {
      if (!this.canPatch()) {
        return;
      }
      const keyStr = String(key);
      if (section === "account" && keyStr.startsWith("permissions.")) {
        const permKey = keyStr.slice("permissions.".length);
        void this.sendPatch({ enforced: { account: { permissions: { [permKey]: value } } } });
        return;
      }
      void this.sendPatch({ enforced: { [section]: { [key]: value } } });
    },
    onEnforcedChange({ section, field, value }) {
      this.patchEnforcedFlag(section, field, value);
    },
    onPreferenceSectionChange({ section, field }) {
      if (!this.canPatch() || !section || !field) {
        return;
      }
      const sectionData = getObjectProperty(this.values, section);
      if (!sectionData) {
        return;
      }
      let value = getObjectProperty(sectionData, field);
      if (section === "preview" && typeof value === "boolean") {
        value = this.normalizePreviewBool(value);
      }
      void this.sendPatch({ [section]: { [field]: value } });
    },
    onAccountFieldChange(field) {
      if (!this.canPatch() || !field) {
        return;
      }
      if (String(field).startsWith("permissions.")) {
        const permKey = String(field).slice("permissions.".length);
        void this.sendPatch({
          account: {
            permissions: {
              [permKey]: getObjectProperty(this.values.account.permissions, permKey),
            },
          },
        });
        return;
      }
      void this.sendPatch({
        account: {
          [field]: getObjectProperty(this.values.account, field),
        },
      });
    },
  },
};
</script>

<style scoped>
.user-defaults-prompt {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.user-defaults-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.loading-hint {
  opacity: 0.7;
}

.lock-message {
  margin: 0 0 1rem;
  padding: 0.75rem 1rem;
  border-radius: 0.25rem;
  background: color-mix(in srgb, var(--primaryColor) 12%, transparent);
  color: var(--textPrimary);
}

.user-defaults-readonly {
  pointer-events: none;
  opacity: 0.65;
}
.user-defaults-prompt :deep(.settings-group) {
  margin-bottom: 0.75rem;
}
</style>
