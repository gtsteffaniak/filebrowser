<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>{{ $t("settings.api") }}</h2>
      <div>
        <button @click.prevent="createPrompt" class="button">
          {{ $t("buttons.new") }}
        </button>
      </div>
    </div>
    <div class="card-content full" v-if="Object.keys(links).length > 0">
      <p>
        API keys are based on the user that creates the. See
        <a class="link" href="swagger/index.html">swagger page</a> for how to use them.
        Keys are associated with your user and the user must have access to the permission
        level you want to use the key with.
      </p>
      <table>
        <tr>
          <th>Name</th>
          <th>Created</th>
          <th>Expires</th>
          <th>{{ $t("settings.permissions") }}</th>
          <th>Actions</th>
        </tr>

        <tr v-for="(link, name) in links" :key="name">
          <td>{{ name }}</td>
          <td>{{ formatTime(link.created) }}</td>
          <td>{{ formatTime(link.expires) }}</td>
          <td>
            <span
              v-for="(value, perm) in link.Permissions"
              :key="perm"
              :title="`${perm}: ${value ? 'Enabled' : 'Disabled'}`"
              class="clickable"
              @click.prevent="infoPrompt(name, link)"
            >
              {{ showResult(value) }}
            </span>
          </td>
          <td class="small">
            <button class="action" @click.prevent="infoPrompt(name, link)">
              <i class="material-icons">info</i>
            </button>
          </td>
          <td class="small">
            <button
              class="action copy-clipboard"
              :data-clipboard-text="link.key"
              :aria-label="$t('buttons.copyToClipboard')"
              :title="$t('buttons.copyToClipboard')"
            >
              <i class="material-icons">content_paste</i>
            </button>
          </td>
        </tr>
      </table>
    </div>
    <h2 class="message" v-else>
      <i class="material-icons">sentiment_dissatisfied</i>
      <span>{{ $t("files.lonely") }}</span>
    </h2>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { usersApi } from "@/api";
import { state, mutations, getters } from "@/store";
import Clipboard from "clipboard";
import Errors from "@/views/Errors.vue";

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
    };
  },
  async created() {
    mutations.setLoading("shares", true);
    try {
      // Fetch the API keys from the specified endpoint
      this.links = await usersApi.getApiKeys(); // Updated to the correct API endpoint
    } catch (e) {
      this.error = e;
    } finally {
      mutations.setLoading("shares", false);
    }
  },
  mounted() {
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      notify.showSuccess("Copied API Key!");
    });
  },
  beforeUnmount() {
    this.clip.destroy();
  },
  computed: {
    settings() {
      return state.settings;
    },
    active() {
      return state.activeSettingsView === "shares-main";
    },
    user() {
      return state.user;
    },
    loading() {
      return getters.isLoading();
    },
  },
  methods: {
    showResult(value) {
      return value ? "✓" : "✗";
    },
    createPrompt() {
      mutations.showHover({ name: "CreateApi", props: { user: this.user } });
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
  color: #888; /* Styling for the placeholder text */
}

.permissions-list {
  display: none;
  position: absolute;
  top: 100%; /* Position the popup below the cell */
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
