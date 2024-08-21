<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card" id="shares-main" :class="{active:active}">
    <div class="card-title">
      <h2>{{ $t("settings.shareManagement") }}</h2>
    </div>

    <div class="card-content full" v-if="links.length > 0">
      <table>
        <tr>
          <th>{{ $t("settings.path") }}</th>
          <th>{{ $t("settings.shareDuration") }}</th>
          <th v-if="user.perm.admin">{{ $t("settings.username") }}</th>
          <th></th>
          <th></th>
        </tr>

        <tr v-for="link in links" :key="link.hash">
          <td>
            <a :href="buildLink(link)" target="_blank">{{ link.path }}</a>
          </td>
          <td>
            <template v-if="link.expire !== 0">{{ humanTime(link.expire) }}</template>
            <template v-else>{{ $t("permanent") }}</template>
          </td>
          <td v-if="user.perm.admin">{{ link.username }}</td>
          <td class="small">
            <button
              class="action"
              @click="deleteLink($event, link)"
              :aria-label="$t('buttons.delete')"
              :title="$t('buttons.delete')"
            >
              <i class="material-icons">delete</i>
            </button>
          </td>
          <td class="small">
            <button
              class="action copy-clipboard"
              :data-clipboard-text="buildLink(link)"
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
import { showSuccess, showError } from "@/notify";
import { share as api, users } from "@/api";
import { state, mutations } from "@/store";
import { fromNow } from "@/utils/moment";
import Clipboard from "clipboard";
import Errors from "@/views/Errors.vue";

export default {
  name: "shares",
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
    mutations.setLoading(true);

    try {
      let links = await api.list();
      if (state.user.perm.admin) {
        let userMap = new Map();
        for (let user of await users.getAllUsers()) userMap.set(user.id, user.username);
        for (let link of links)
          link.username = userMap.has(link.userID) ? userMap.get(link.userID) : "";
      }
      this.links = links;
    } catch (e) {
      this.error = e;
    } finally {
      mutations.setLoading(false);
    }
  },
  mounted() {
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      showSuccess(this.$t("success.linkCopied"));
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
      return state.loading;
    },
  },
  methods: {
    deleteLink: async function (event, link) {
      event.preventDefault();
      mutations.showHover({
        name: "share-delete",
        confirm: () => {
          mutations.closeHovers();

          try {
            api.remove(link.hash);
            this.links = this.links.filter((item) => item.hash !== link.hash);
            showSuccess(this.$t("settings.shareDeleted"));
          } catch (e) {
            showError(e);
          }
        },
      });
    },
    humanTime(time) {
      return fromNow(time * 1000, state.user.locale);
    },
    buildLink(share) {
      return api.getShareURL(share);
    },
  },
};
</script>
