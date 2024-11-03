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
    <div class="card-content full" v-if="links.length > 0">
      <p>
        API keys are based on the user that creates the. See
        <a class="link" href="/swagger/index.html">swagger page</a> for how to use them
      </p>
      <table>
        <tr>
          <th>Copy Token</th>
          <th>Name</th>
          <th>Key Duration</th>
          <th>Expires At</th>
          <th>{{ $t("settings.permissions") }}</th>
          <th>Delete</th>
        </tr>

        <tr v-for="link in links" :key="link.key">
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
          <td>{{ link.name }}</td>
          <td>{{ humanTime(link.duration) }}</td>
          <td>{{ formatExpiresAt(link.expiresAt) }}</td>
          <td>
            <div class="permissions-cell">
              <!-- Placeholder text, always visible -->
              <span class="permissions-placeholder">Hover to view permissions</span>

              <!-- Permissions list, shown only on hover -->
              <div class="permissions-list">
                <div v-for="(value, perm) in link.Permissions" :key="perm">
                  {{ perm }}: {{ value }}
                </div>
              </div>
            </div>
          </td>
          <td class="small">
            <button class="action delete">
              <i class="material-icons">delete</i>
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
import { users } from "@/api";
import { state, mutations, getters } from "@/store";
import { fromNow } from "@/utils/moment";
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
      links: [],
      clip: null,
    };
  },
  async created() {
    mutations.setLoading("shares", true);

    try {
      // Fetch the API keys from the specified endpoint
      this.links = await users.getApiKeys(); // Updated to the correct API endpoint
    } catch (e) {
      this.error = e;
    } finally {
      mutations.setLoading("shares", false);
    }
  },
  mounted() {
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      notify.showSuccess(this.$t("success.linkCopied"));
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
    createPrompt() {
      mutations.showHover({ name: "CreateApi", props: { user: this.user } });
    },
    humanTime(time) {
      return fromNow(time, state.user.locale); // Adjust time as necessary
    },
    formatExpiresAt(expiresAt) {
      return new Date(expiresAt * 1000).toLocaleString(); // Format the expiresAt value
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
