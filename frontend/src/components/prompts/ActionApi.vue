<template>
  <div class="card-title">
    <h2>{{ $t('api.title') }}</h2> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
  </div>

  <div class="card-content api-content">
    <!-- API Token Section -->
    <div class="api-section">
      <button
        class="action copy-clipboard api-key-button"
        :data-clipboard-text="name"
        :aria-label="$t('buttons.copyToClipboard')"
        :title="$t('buttons.copyToClipboard')"
      >
        <span class="api-key-name">{{ name }}</span>
        <i class="material-icons">content_paste</i>
      </button>
      
      <button
        class="action copy-clipboard api-key-value-button"
        :data-clipboard-text="info.key"
        :aria-label="$t('api.clickToCopyKey')"
        :title="$t('api.clickToCopyKey')"
      >
        <span class="api-key-value">{{ $t('api.clickToCopyKey') }}</span>
        <i class="material-icons">content_paste</i>
      </button>
    </div>

    <!-- Information Section -->
    <div class="api-section">
      <h3 class="section-title">{{ $t('general.info') }}</h3>
      <div class="info-item">
        <span class="info-label">{{ $t('api.createdAt') }}</span>
        <span class="info-value">{{ formatTime(info.created) }}</span>
      </div>
      <div class="info-item">
        <span class="info-label">{{ $t('api.expiresAt') }}</span>
        <span class="info-value">{{ formatTime(info.expires) }}</span>
      </div>
    </div>

    <!-- Token Type or Permissions Section -->
    <div class="api-section" v-if="info.minimal">
      <p class="minimal-info">{{ $t('api.minimalInfo') }}</p>
    </div>

    <div class="api-section" v-else>
      <h3 class="section-title">{{ $t('api.permissions') }}</h3>
      <div class="permissions-grid">
        <div
          v-for="(isEnabled, permission) in info.Permissions"
          :key="permission"
          class="permission-item"
        >
          <span class="permission-name">{{ permission }}</span>
          <span class="permission-status" :class="{ enabled: isEnabled, disabled: !isEnabled }">
            {{ isEnabled ? '✓' : '✗' }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          </span>
        </div>
      </div>
    </div>
  </div>

  <div class="card-action">
    <button
      class="button button--flat button--grey"
      @click="closeHovers"
      :aria-label="$t('general.close')"
      :title="$t('general.close')"
    >
      {{ $t('general.close') }}
    </button>
    <button
      class="button button--flat button--red"
      @click="deleteApi"
      :title="$t('general.delete')"
    >
      {{ $t('general.delete') }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";
import { eventBus } from "@/store/eventBus";

export default {
  name: "ActionApi",
  props: {
    name: {
      type: String,
      required: true,
    },
    info: {
      type: Object,
      required: true,
    },
  },
  methods: {
    formatTime(timestamp) {
      return new Date(timestamp * 1000).toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    },
    closeHovers() {
      mutations.closeHovers();
    },
    async deleteApi() {
      // Dummy delete function, to be filled in later
      try {
        usersApi.deleteApiKey({ name: this.name });
        // Emit event to refresh API tokens list
        setTimeout(() => {
          eventBus.emit('apiKeysChanged');
        }, 10);
        mutations.closeHovers();
        notify.showSuccessToast(this.$t("api.apiKeyDeleted"));
      } catch (error) {
        console.error(error);
      }
    },
  },
};
</script>
<style scoped>
.api-content {
  max-height: 70vh;
  overflow-y: auto;
}

.api-section {
  margin-bottom: 2em;
}

.api-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 0.95em;
  font-weight: 600;
  color: var(--textPrimary);
  margin: 0 0 0.75em 0;
  padding-bottom: 0.5em;
  border-bottom: 1px solid var(--divider);
}

.api-key-title {
  margin-top: 1.5em;
}

.api-key-button,
.api-key-value-button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.75em 1em;
  margin-top: 0.5em;
  background-color: var(--surfaceSecondary);
  border: 1px solid var(--divider);
  border-radius: 4px;
  transition: background-color 0.2s;
  cursor: pointer;
}

.api-key-button:hover,
.api-key-value-button:hover {
  background-color: var(--surfaceTertiary);
}

.api-key-name {
  font-family: monospace;
  font-size: 0.9em;
  font-weight: 500;
  color: var(--textPrimary);
  word-break: break-all;
  flex: 1;
  margin-right: 0.5em;
}

.api-key-value {
  font-size: 0.9em;
  font-weight: 500;
  color: var(--textSecondary);
  flex: 1;
  margin-right: 0.5em;
  font-style: italic;
}

.api-key-button .material-icons,
.api-key-value-button .material-icons {
  font-size: 1.2em;
  color: var(--textSecondary);
  flex-shrink: 0;
}

.info-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75em 0.5em;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.info-item:hover {
  background-color: var(--surfaceSecondary);
}

.info-label {
  font-weight: 500;
  color: var(--textSecondary);
  font-size: 0.9em;
}

.info-value {
  color: var(--textPrimary);
  font-size: 0.9em;
  text-align: right;
}

.minimal-info {
  font-style: italic;
  color: var(--textSecondary);
  margin-top: 0.75em;
  padding: 0.75em;
  background-color: var(--surfaceSecondary);
  border-radius: 4px;
  line-height: 1.5;
  font-size: 0.9em;
}

.permissions-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 0.5em;
  margin-top: 0.5em;
}

.permission-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75em;
  background-color: var(--surfaceSecondary);
  border: 1px solid var(--divider);
  border-radius: 4px;
  transition: background-color 0.2s;
}

.permission-item:hover {
  background-color: var(--surfaceTertiary);
}

.permission-name {
  font-size: 0.9em;
  color: var(--textPrimary);
  flex: 1;
}

.permission-status {
  font-size: 1.1em;
  font-weight: 600;
  min-width: 1.5em;
  text-align: center;
}

.permission-status.enabled {
  color: #4caf50;
}

.permission-status.disabled {
  color: #f44336;
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .permissions-grid {
    grid-template-columns: 1fr;
  }

  .info-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25em;
  }

  .info-value {
    text-align: left;
  }
}
</style>
