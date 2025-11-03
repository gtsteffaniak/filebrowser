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

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')"
      >
        {{ $t('buttons.close') }}
      </button>
      <button
        class="button button--flat button--red"
        @click="deleteApi"
        :title="$t('buttons.delete')"
      >
        {{ $t('buttons.delete') }}
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
    deleteApi() {
      // Dummy delete function, to be filled in later
      try {
        usersApi.deleteApiKey({ name: this.name });
        // Emit event to refresh API keys list
        eventBus.emit('apiKeysChanged');
        notify.showSuccess("API key deleted!");
        mutations.closeHovers();
      } catch (error) {
        console.error(error);
      }
    },
  },
};
</script>
