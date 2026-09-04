<template>
  <div class="card-content">
    <div v-if="loading" class="loading-hint">{{ $t("general.loading") }}</div>

    <ShareOptionsForm
      v-else
      v-model="values"
      enforceable
      :enforced="enforced"
      :read-only-source="false"
      @enforced-change="onEnforcedChange"
      @customize-sidebar-links="openSidebarLinksCustomization"
    />
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat"
      :disabled="saving"
      @click="close"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="saving || loading"
      @click="save"
      :aria-label="$t('general.save')"
      :title="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import { getShareDefaults, patchShareDefaults } from "@/api/settings";
import ShareOptionsForm from "@/components/share/ShareOptionsForm.vue";
import {
  emptyShareDefaultsEnforced,
  emptyShareDefaultsValues,
  shareDefaultsFromApi,
  shareDefaultsToApiPayload,
} from "@/utils/shareDefaultsForm";

export default {
  name: "share-defaults",
  components: {
    ShareOptionsForm,
  },
  data() {
    return {
      loading: true,
      saving: false,
      values: emptyShareDefaultsValues(),
      enforced: emptyShareDefaultsEnforced(),
    };
  },
  mounted() {
    eventBus.on("shareSidebarLinksUpdated", this.handleSidebarLinksUpdate);
    void this.load();
  },
  beforeUnmount() {
    eventBus.off("shareSidebarLinksUpdated", this.handleSidebarLinksUpdate);
  },
  methods: {
    close() {
      mutations.closeTopPrompt();
    },
    applyResponse(data) {
      this.values = shareDefaultsFromApi(data.values || {});
      this.enforced = {
        ...emptyShareDefaultsEnforced(),
        ...(data.enforced || {}),
      };
    },
    async load() {
      this.loading = true;
      try {
        const data = await getShareDefaults();
        this.applyResponse(data);
      } catch (e) {
        notify.showError(e);
        console.error(e);
      } finally {
        this.loading = false;
      }
    },
    onEnforcedChange(field, value) {
      this.enforced = {
        ...this.enforced,
        [field]: value,
      };
    },
    handleSidebarLinksUpdate(data) {
      if (!data?.sidebarLinks) {
        return;
      }
      this.values = {
        ...this.values,
        sidebarLinks: [...data.sidebarLinks],
      };
    },
    openSidebarLinksCustomization() {
      mutations.showPrompt({
        name: "sidebarlinks",
        props: {
          context: "share",
          shareData: {
            hash: "share-defaults",
            sidebarLinks: this.values.sidebarLinks,
          },
        },
      });
    },
    async save() {
      if (this.saving || this.loading) {
        return;
      }
      this.saving = true;
      try {
        await patchShareDefaults(shareDefaultsToApiPayload(this.values));
        await patchShareDefaults({ enforced: this.enforced });
        notify.showSuccess(this.$t("general.saved"));
        this.close();
      } catch (e) {
        notify.showError(e);
        console.error(e);
        await this.load();
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.loading-hint {
  opacity: 0.7;
}
</style>
