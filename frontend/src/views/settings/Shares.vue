<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("settings.shareManagement") }}</h2>
  </div>

  <div class="card-content full" v-if="links.length > 0">
    <table aria-label="Shares">
      <thead>
        <tr>
          <th>{{ $t("general.hash") }}</th>
          <th>{{ $t("general.path") }}</th>
          <th>{{ $t("general.expiration") }}</th>
          <th>{{ $t("general.downloads") }}</th>
          <th>{{ $t("general.username") }}</th>
          <th></th>
          <th></th>
          <th></th>
          <th></th>
          <th></th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="item in links" :key="item.hash">
          <td>{{ item.hash }}</td>
          <td>
            <a :href="buildLink(item)" target="_blank">{{ item.path }}</a>
          </td>
          <td>
            <template v-if="item.expire !== 0">{{ humanTime(item.expire) }}</template>
            <template v-else>{{ $t("general.permanent") }}</template>
          </td>
          <td>
            <template v-if="item.downloadsLimit && item.downloadsLimit > 0">{{ item.downloads }} / {{ item.downloadsLimit }}</template> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            <template v-else>{{ item.downloads }}</template>
          </td>
          <td>{{ item.username }}</td>
          <td class="small">
            <i v-if="!item.pathExists" class="material-icons warning-icon" :title="$t('messages.pathNotFound')">warning</i>
          </td>
          <td class="small">
            <button class="action" @click="editLink(item)" :aria-label="$t('general.edit')"
              :title="$t('general.edit')">
              <i class="material-icons">edit</i>
            </button>
          </td>
          <td class="small">
            <button class="action" @click="deleteLink($event, item)" :aria-label="$t('general.delete')"
              :title="$t('general.delete')">
              <i class="material-icons">delete</i>
            </button>
          </td>
          <td class="small">
            <button class="action copy-clipboard" :data-clipboard-text="buildLink(item)"
              :aria-label="$t('buttons.copyToClipboard')" :title="$t('buttons.copyToClipboard')">
              <i class="material-icons">content_paste</i>
            </button>
          </td>
          <td class="small">
            <button :disabled="item.shareType == 'upload'" class="action copy-clipboard" :data-clipboard-text="item.downloadURL" v-if="item.downloadURL"
              :aria-label="$t('buttons.copyDownloadLinkToClipboard')" :title="$t('buttons.copyDownloadLinkToClipboard')">
              <i class="material-icons">content_paste_go</i>
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
import { fromNow } from '@/utils/moment';
import { eventBus } from "@/store/eventBus";

export default {
  name: "shares",
  components: {
    Errors,
  },
  data: function () {
    return {
      /** @type {any} */
      error: null,
      /** @type {any[]} */
      links: [],
      /** @type {any} */
      clip: null,
    };
  },
  async created() {
    await this.reloadShares();
  },
  mounted() {
    this.initClipboard();
    // Listen for share changes
    eventBus.on('sharesChanged', this.reloadShares);
  },
  beforeUnmount() {
    // Clean up clipboard
    if (this.clip) {
      this.clip.destroy();
    }
    // Clean up event listener
    eventBus.off('sharesChanged', this.reloadShares);
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
    async reloadShares() {
      mutations.setLoading("shares", true);
      try {
        let links = await shareApi.list();
        if (links.length === 0) {
          this.links = [];
          return;
        }
        this.links = links;
        this.error = null; // Clear any previous errors
      this.$nextTick(() => {
        this.initClipboard();
      });
      } catch (e) {
        this.error = e;
        console.error(e);
      } finally {
        mutations.setLoading("shares", false);
      }
    },
    initClipboard() {
      // First destroy any existing clipboard
      if (this.clip) {
        this.clip.destroy();
      }
      // Create new clipboard
      this.clip = new Clipboard(".copy-clipboard");
      this.clip.on("success", () => {
        notify.showSuccessToast(this.$t("success.linkCopied"));
      });
      this.clip.on("error", () => {
        notify.showErrorToast(this.$t("prompts.copyToClipboardFailed"));
      });
    },
    editLink(item) {
      mutations.showHover({
        name: "share",
        props: {
          editing: true,
          link: item,
        },
      });
    },
    /**
     * @param {any} event
     * @param {any} item
     */
    deleteLink: async function (event, item) {
      mutations.showHover({
        name: "share-delete",
        props: { path: item.path },
        confirm: () => {
          mutations.closeHovers();
          try {
            shareApi.remove(item.hash);
            this.links = this.links.filter((link) => link.hash !== item.hash);
            notify.showSuccessToast(this.$t("settings.shareDeleted"));
          } catch (e) {
            console.error(e);
          }
        },
      });
    },
    /**
     * @param {any} time
     */
    humanTime(time) {
      return fromNow(time);
    },
    /**
     * @param {any} share
     */
    buildLink(share) {
      return share.shareURL;
    },
  },
};
</script>

<style scoped>
tr > td,
tr > th {
  text-align: center;
}
</style>