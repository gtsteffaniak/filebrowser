<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("api.title") }}</h2>
    <div>
      <button @click.prevent="createPrompt" class="button">
        {{ $t("buttons.new") }}
      </button>
    </div>
  </div>

  <div class="card-content full" v-if="Object.keys(links).length > 0">
    <p>
      {{ $t("api.description") }}
      <a class="link" href="swagger/index.html">{{ $t("api.swaggerLinkText") }}</a>
    </p>

    <table aria-label="API Keys">
      <thead>
        <tr>
          <th>{{ $t("api.name") }}</th>
          <th>{{ $t("api.created") }}</th>
          <th>{{ $t("api.expires") }}</th>
          <th>{{ $t("settings.permissions-name") }}</th>
          <th>{{ $t("api.actions") }}</th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="(link, name) in links" :key="name">
          <td>{{ name }}</td>
          <td>{{ formatTime(link.created) }}</td>
          <td>{{ formatTime(link.expires) }}</td>
          <td>
            <span v-for="(value, permission) in link.Permissions" :key="permission"
              :title="`${permission}: ${value ? $t('api.enabled') : $t('api.disabled')}`" class="clickable"
              @click.prevent="infoPrompt(name, link)">
              {{ showResult(value) }}
            </span>
          </td>
          <td class="small">
            <button class="action" @click.prevent="infoPrompt(name, link)">
              <i class="material-icons">info</i>
            </button>
          </td>
          <td class="small">
            <button class="action copy-clipboard" :data-clipboard-text="link.key"
              :aria-label="$t('buttons.copyToClipboard')" :title="$t('buttons.copyToClipboard')">
              <i class="material-icons">content_paste</i>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <h2 class="message" v-else-if="!loading">
    <i class="material-icons">sentiment_dissatisfied</i>
    <span>{{ $t("files.lonely") }}</span>
  </h2>
</template>

<script>
import { notify } from "@/notify";
import { usersApi } from "@/api";
import { state, mutations, getters } from "@/store";
import Clipboard from "clipboard";
import Errors from "@/views/Errors.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "api",
  components: {
    Errors,
  },
  data: function () {
    return {
      error: null,
      links: {},
      clip: null,
      user: {
        permissions: { ...state.user.permissions}
      },
    };
  },
  async created() {
    await this.reloadApiKeys();
  },
  mounted() {
    // Listen for API key changes
    eventBus.on('apiKeysChanged', this.reloadApiKeys);
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      notify.showSuccess("Copied API Key!");
    });
  },
  beforeUnmount() {
    // Clean up event listener
    eventBus.removeEventListener('apiKeysChanged', this.reloadApiKeys);
    this.clip.destroy();
  },
  computed: {
    settings() {
      return state.settings;
    },
    active() {
      return state.activeSettingsView === "shares-main";
    },
    loading() {
      return getters.isLoading();
    },
  },
  methods: {
    async reloadApiKeys() {
      mutations.setLoading("api", true);
      try {
        // Fetch the API keys from the specified endpoint
        this.links = await usersApi.getApiKeys();
        this.error = null; // Clear any previous errors
      } catch (e) {
        this.error = e;
      } finally {
        mutations.setLoading("api", false);
      }
    },
    showResult(value) {
      return value ? "✓" : "✗";
    },
    createPrompt() {
      mutations.showHover({
        name: "CreateApi",
        props: { permissions: this.user.permissions },
      });
    },
    infoPrompt(name, info) {
      mutations.showHover({ name: "ActionApi", props: { name: name, info: info } });
    },
    formatTime(time) {
      return new Date(time * 1000).toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    },
  },
};
</script>
<style>
.permissions-cell {
  position: relative;
  display: inline-block;
}

.permissions-placeholder {
  color: #888;
  /* Styling for the placeholder text */
}

.permissions-list {
  display: none;
  position: absolute;
  top: 100%;
  /* Position the popup below the cell */
  left: 0;
  background-color: white;
  border: 1px solid #ccc;
  padding: 8px;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  z-index: 10;
  width: max-content;
}

.permissions-cell:hover .permissions-list {
  display: block;
}
</style>
