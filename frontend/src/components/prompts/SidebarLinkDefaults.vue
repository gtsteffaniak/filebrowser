<template>
  <div v-if="loading" class="card-content prompt-panel">
    <div class="loading-hint">{{ $t("general.loading") }}</div>
  </div>

  <SidebarLinksEditor
    v-else
    embedded
    mode="defaults"
    :model-value="items"
    :description="$t('sidebar.sidebarLinkDefaultsHelp')"
    :disabled="!canPatch()"
    :show-prompt-actions="false"
    @update:model-value="onItemsUpdate"
    @change="save"
  />
</template>

<script>
import { notify } from "@/notify";
import { getSidebarLinkDefaults, patchSidebarLinkDefaults } from "@/api/settings";
import SidebarLinksEditor from "@/components/sidebar/SidebarLinksEditor.vue";

export default {
  name: "sidebar-link-defaults",
  components: {
    SidebarLinksEditor,
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
            enabled: !!item.enabled,
            enforced: !!item.enforced,
            link: { ...item.link },
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
        const data = await getSidebarLinkDefaults();
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
    async save() {
      if (!this.canPatch()) {
        return;
      }
      this.saving = true;
      try {
        const data = await patchSidebarLinkDefaults({ items: this.items });
        this.applyItems(data);
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
