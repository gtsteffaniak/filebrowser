<template>
  <div v-if="loading" class="card-content">
    <div class="loading-hint">{{ $t("general.loading") }}</div>
  </div>

  <template v-else>
    <SidebarLinksEditor
      embedded
      mode="defaults"
      :model-value="items"
      :description="$t('sidebar.sidebarLinkDefaultsHelp')"
      :disabled="!canPatch()"
      :show-prompt-actions="false"
      @update:model-value="onItemsUpdate"
      @sub-flow-change="linksInSubFlow = $event"
      @save="save"
    />
    <div v-if="!linksInSubFlow" class="card-actions">
      <button
        type="button"
        class="button button--flat"
        :disabled="!canPatch()"
        @click="closeTopPrompt"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        class="button button--flat button--blue"
        :disabled="!canPatch()"
        @click="save"
      >
        {{ $t("general.save") }}
      </button>
    </div>
  </template>
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { getSidebarLinkDefaults, patchSidebarLinkDefaults } from "@/api/settings";
import SidebarLinksEditor from "@/components/sidebar/SidebarLinksEditor.vue";

export default {
  name: "sidebar-link-defaults",
  components: {
    SidebarLinksEditor,
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
  },
  data() {
    return {
      loading: true,
      saving: false,
      hydrating: false,
      items: [],
      linksInSubFlow: false,
    };
  },
  mounted() {
    void this.load();
  },
  methods: {
    canPatch() {
      return !this.loading && !this.saving && !this.hydrating;
    },
    closeTopPrompt() {
      mutations.closeTopPrompt();
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
