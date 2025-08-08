<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("settings.shareManagement") }}</h2>
  </div>

  <div class="card-content full" v-if="links.length > 0">
    <table aria-label="Shares">
      <thead>
        <tr>
          <th>{{ $t("settings.path") }}</th>
          <th>{{ $t("settings.shareDuration") }}</th>
          <th v-if="user.permissions.admin">{{ $t("settings.username") }}</th>
          <th></th>
          <th></th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="link in links" :key="link.hash">
          <td>
            <a :href="buildLink(link)" target="_blank">{{ link.path }}</a>
          </td>
          <td>
            <template v-if="link.expire !== 0">{{ humanTime(link.expire) }}</template>
            <template v-else>{{ $t("permanent") }}</template>
          </td>
          <td v-if="user.permissions.admin">{{ link.username }}</td>
          <td class="small">
            <button class="action" @click="deleteLink($event, link)" :aria-label="$t('buttons.delete')"
              :title="$t('buttons.delete')">
              <i class="material-icons">delete</i>
            </button>
          </td>
          <td class="small">
            <button class="action copy-clipboard" :data-clipboard-text="buildLink(link)"
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
import { shareApi } from "@/api";
import { state, mutations, getters } from "@/store";
import Clipboard from "clipboard";
import Errors from "@/views/Errors.vue";
import { fromNow } from '@/utils/moment'

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
    mutations.setLoading("shares", true);
    try {
      let links = await shareApi.list();
      if (links.length === 0) {
        this.links = [];
        return;
      }
      this.links = links;
    } catch (e) {
      this.error = e;
      notify.showError(e);
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
    deleteLink: async function (event, link) {
      mutations.showHover({
        name: "share-delete",
        props: { path: link.path },
        confirm: () => {
          mutations.closeHovers();

          try {
            shareApi.remove(link.hash);
            this.links = this.links.filter((item) => item.hash !== link.hash);
            notify.showSuccess(this.$t("settings.shareDeleted"));
          } catch (e) {
            notify.showError(e);
          }
        },
      });
    },
    humanTime(time) {
      return fromNow(time);
    },
    buildLink(share) {
      return shareApi.getShareURL(share);
    },
  },
};
</script>
