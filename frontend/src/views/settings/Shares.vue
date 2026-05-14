<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("settings.shareManagement") }}</h2>
  </div>

  <div class="card-content full">
    <settings-table
      :columns="sharesTableColumns"
      :items="links"
      item-key="hash"
      default-sort-key="path"
      :aria-label="$t('settings.shareManagement')"
      :loading="loading"
    >
      <template #cell-path="{ row }">
        <a :href="buildLink(row)" target="_blank" rel="noopener noreferrer">{{ row.path }}</a>
      </template>
      <template #cell-expire="{ row }">
        <template v-if="row.expire !== 0">{{ humanTime(row.expire) }}</template>
        <template v-else>{{ $t("general.permanent") }}</template>
      </template>
      <template #cell-downloads="{ row }">
        <template v-if="row.downloadsLimit && row.downloadsLimit > 0">{{ row.downloads }} / {{ row.downloadsLimit }}</template> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <template v-else>{{ row.downloads }}</template>
      </template>
      <template #cell-warning="{ row }">
        <i
          v-if="!row.pathExists"
          class="material-symbols warning-icon"
          :title="$t('messages.pathNotFound')"
        >warning</i>
      </template>
      <template #cell-edit="{ row }">
        <button class="action" @click="editLink(row)" :aria-label="$t('general.edit')"
          :title="$t('general.edit')"
        >
          <i class="material-symbols">edit</i>
        </button>
      </template>
      <template #cell-delete="{ row }">
        <button class="action" @click="deleteLink($event, row)" :aria-label="$t('general.delete')"
          :title="$t('general.delete')"
        >
          <i class="material-symbols">delete</i>
        </button>
      </template>
      <template #cell-copyShare="{ row }">
        <button class="action" @click.stop="copyToClipboard(buildLink(row))"
          :aria-label="$t('buttons.copyToClipboard')" :title="$t('buttons.copyToClipboard')"
        >
          <i class="material-symbols">content_paste</i>
        </button>
      </template>
      <template #cell-copyDownload="{ row }">
        <button
          :disabled="row.shareType === 'upload'"
          class="action"
          v-if="row.downloadURL"
          @click.stop="copyToClipboard(row.downloadURL)"
          :aria-label="$t('buttons.copyDownloadLinkToClipboard')"
          :title="$t('buttons.copyDownloadLinkToClipboard')"
        >
          <i class="material-symbols">content_paste_go</i>
        </button>
      </template>
    </settings-table>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { shareApi } from "@/api";
import { state, mutations } from "@/store";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import { fromNow } from '@/utils/moment';
import { eventBus } from "@/store/eventBus";
import { copyToClipboard } from "@/utils/clipboard";

export default {
  name: "shares",
  components: {
    Errors,
    SettingsTable,
  },
  data: function () {
    return {
      /** @type {any} */
      error: null,
      /** @type {any[]} */
      links: [],
      /** Local fetch state; avoids global Settings overlay spinner (table shows its own). */
      loading: true,
    };
  },
  async created() {
    await this.reloadShares();
  },
  mounted() {
    // Listen for share changes
    eventBus.on('sharesChanged', this.reloadShares);
  },
  beforeUnmount() {
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
    sharesTableColumns() {
      return [
        {
          key: "hash",
          label: this.$t("general.hash"),
          sortable: true,
          align: "center",
        },
        {
          key: "path",
          label: this.$t("general.path"),
          sortable: true,
          align: "center",
        },
        {
          key: "expire",
          label: this.$t("general.expiration"),
          sortable: true,
          sortFn: (a, b) => (a.expire ?? 0) - (b.expire ?? 0),
          align: "center",
        },
        {
          key: "downloads",
          label: this.$t("general.downloads"),
          sortable: true,
          sortFn: (a, b) => (a.downloads ?? 0) - (b.downloads ?? 0),
          align: "center",
        },
        {
          key: "username",
          label: this.$t("general.username"),
          sortable: true,
          align: "center",
        },
        { key: "warning", label: "", narrow: true, align: "center" },
        { key: "edit", label: "", narrow: true, align: "center" },
        { key: "delete", label: "", narrow: true, align: "center" },
        { key: "copyShare", label: "", narrow: true, align: "center" },
        { key: "copyDownload", label: "", narrow: true, align: "center" },
      ];
    },
  },
  methods: {
    async copyToClipboard(text) {
      await copyToClipboard(text);
    },
    async reloadShares() {
      this.loading = true;
      try {
        let links = await shareApi.list();
        if (links.length === 0) {
          this.links = [];
          return;
        }
        this.links = links;
        this.error = null; // Clear any previous errors
      } catch (e) {
        this.error = e;
        console.error(e);
      } finally {
        this.loading = false;
      }
    },
    editLink(item) {
      mutations.showPrompt({
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
      mutations.showPrompt({
        name: "generic",
        props: {
          title: this.$t("general.delete"),
          body: this.$t("prompts.deleteMessageShare", { path: item.path }),
          buttons: [
            {
              label: this.$t("general.delete"),
              action: () => {
                try {
                  shareApi.remove(item.hash);
                  this.links = this.links.filter((link) => link.hash !== item.hash);
                  notify.showSuccessToast(this.$t("settings.shareDeleted"));
                  mutations.closeTopPrompt();
                } catch (e) {
                  console.error(e);
                  notify.showErrorToast(this.$t("share.deleteFailed"));
                }
              },
            },
          ],
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


