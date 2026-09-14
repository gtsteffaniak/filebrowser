<template>
  <div v-if="loading" class="card-content prompt-panel">
    <div class="loading-hint">{{ $t("general.loading") }}</div>
  </div>

  <ToolsAccessEditor
    v-else
    embedded
    mode="defaults"
    :model-value="items"
    :description="$t('tools.toolAccessDefaultsHelp')"
    :disabled="!canPatch()"
    :show-prompt-actions="true"
    @update:model-value="onItemsUpdate"
    @save="save"
    @cancel="closeTopPrompt"
  />
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { getToolAccessDefaults, patchToolAccessDefaults } from "@/api/settings";
import ToolsAccessEditor from "@/components/tools/ToolsAccessEditor.vue";

export default {
  name: "tool-access-defaults",
  components: {
    ToolsAccessEditor,
  },
  data() {
    return {
      loading: true,
      saving: false,
      hydrating: false,
      items: [],
    };
  },
  mounted() {
    void this.load();
  },
  methods: {
    canPatch() {
      return !this.loading && !this.saving && !this.hydrating;
    },
    applyItems(data) {
      this.hydrating = true;
      this.items = Array.isArray(data.items)
        ? data.items.map((item) => ({
            toolId: item.toolId,
            enabled: !!item.enabled,
            enforced: !!item.enforced,
          }))
        : [];
      this.$nextTick(() => {
        this.hydrating = false;
      });
    },
    onItemsUpdate(items) {
      if (this.hydrating) {
        return;
      }
      this.items = items;
    },
    async load() {
      this.loading = true;
      this.hydrating = true;
      try {
        const data = await getToolAccessDefaults();
        this.applyItems(data);
      } catch (e) {
        notify.showError(e);
        console.error(e);
      } finally {
        this.loading = false;
        this.$nextTick(() => {
          this.hydrating = false;
        });
      }
    },
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    async save() {
      if (!this.canPatch()) {
        return;
      }
      this.saving = true;
      try {
        const data = await patchToolAccessDefaults({ items: this.items });
        this.applyItems(data);
        mutations.applyToolAccessDefaultsPolicy(this.items);
        await mutations.syncToolAccessDefaultsPolicy();
        mutations.closeTopPrompt();
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
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
