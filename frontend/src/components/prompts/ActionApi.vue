<template>
    <div class="card-title">
      <h2>{{ $t('api.title') }}:</h2> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </div>

    <div class="card-content">
      <button
        class="action copy-clipboard"
        :data-clipboard-text="info.key"
        :aria-label="$t('buttons.copyToClipboard')"
        :title="$t('buttons.copyToClipboard')"
      >
        {{ $t('api.keyName') }}{{ name }}
        <i class="material-icons">content_paste</i>
      </button>

      <h3>{{ $t('api.createdAt') }}</h3>
      {{ formatTime(info.created) }}
      <h3>{{ $t('api.expiresAt') }}</h3>
      {{ formatTime(info.expires) }}
      <div v-if="info.stateful">
        <h3>{{ $t('api.tokenType') }}</h3>
        <p class="stateful-info">{{ $t('api.statefulTokenInfo') }}</p>
      </div>
      <div v-else>
        <h3>{{ $t('api.permissions') }}</h3>
        <table>
          <tbody>
            <tr v-for="(isEnabled, permission) in info.Permissions" :key="permission">
              <td>{{ permission }}</td>
              <td>{{ isEnabled ? '✓' : '✗' }}</td> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            </tr>
          </tbody>
        </table>
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
        // Emit event to refresh API keys list
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
.stateful-info {
  font-style: italic;
  color: #666;
  margin-top: 0.5em;
}
</style>
