<template>
  <div class="card-content no-buttons prompt-panel sidebar-link-defaults-prompt">
    <p class="defaults-description">{{ $t("settings.sidebarLinkDefaultsDescription") }}</p>

    <YamlEditorPanel
      v-if="yamlMode"
      v-model="yamlText"
      @apply="applyYaml"
      @cancel="yamlMode = false"
    />

    <template v-else>
      <div v-if="loading" class="loading-hint">{{ $t("general.loading") }}</div>
      <div v-else class="defaults-list">
        <div v-for="(item, index) in items" :key="index" class="default-item input">
          <div class="default-item-main">
            <i class="material-symbols default-item-icon">{{ item.link.icon || "link" }}</i>
            <div class="default-item-details">
              <span class="default-item-name">{{ linkLabel(item.link) }}</span>
              <span class="default-item-meta">{{ item.link.category }} · {{ item.link.target || "/" }}</span>
            </div>
          </div>
          <div class="default-item-toggles">
            <label class="default-toggle">
              <span>{{ $t("settings.sidebarLinkDefaultEnabled") }}</span>
              <input
                type="checkbox"
                :checked="item.enabled"
                @change="setFlag(index, 'enabled', $event.target.checked)"
              />
            </label>
            <label class="default-toggle">
              <span>{{ $t("general.enforce", { suffix: "" }) }}</span>
              <input
                type="checkbox"
                :checked="item.enforced"
                @change="setFlag(index, 'enforced', $event.target.checked)"
              />
            </label>
          </div>
          <button
            v-if="!isSourceItem(item)"
            type="button"
            class="action"
            :aria-label="$t('general.delete')"
            @click="removeItem(index)"
          >
            <i class="material-symbols">delete</i>
          </button>
        </div>
      </div>

      <div v-if="!loading" class="defaults-actions">
        <button type="button" class="button button--flat button--blue" @click="showAddCustom = true">
          <i class="material-symbols">add</i>
          {{ $t("sidebar.addNewLink") }}
        </button>
        <button type="button" class="button button--flat" @click="openYamlEditor">
          {{ $t("settings.editAsYaml") }}
        </button>
      </div>

      <div v-if="showAddCustom" class="add-custom-form">
        <p>{{ $t("sidebar.linkName") }}</p>
        <input v-model="newCustom.name" type="text" class="input" />
        <p>{{ $t("sidebar.linkUrl") }}</p>
        <input v-model="newCustom.target" type="text" class="input" />
        <div class="add-custom-buttons">
          <button type="button" class="button button--flat button--grey" @click="showAddCustom = false">
            {{ $t("general.cancel") }}
          </button>
          <button type="button" class="button button--flat button--blue" :disabled="!newCustom.name || !newCustom.target" @click="addCustomLink">
            {{ $t("general.add") }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { getSidebarLinkDefaults, patchSidebarLinkDefaults } from "@/api/settings";
import YamlEditorPanel from "./YamlEditorPanel.vue";
import yaml from "js-yaml";

export default {
  name: "sidebar-link-defaults",
  components: {
    YamlEditorPanel,
  },
  data() {
    return {
      loading: true,
      saving: false,
      items: [],
      yamlMode: false,
      yamlText: "",
      showAddCustom: false,
      newCustom: { name: "", target: "" },
    };
  },
  mounted() {
    void this.load();
  },
  methods: {
    linkLabel(link) {
      return link.name || link.sourceName || link.target || this.$t("sidebar.customLink");
    },
    isSourceItem(item) {
      const cat = item?.link?.category || "";
      return cat === "source" || cat.startsWith("source-");
    },
    async load() {
      this.loading = true;
      try {
        const data = await getSidebarLinkDefaults();
        this.items = Array.isArray(data.items) ? data.items.map((item) => ({
          enabled: !!item.enabled,
          enforced: !!item.enforced,
          link: { ...item.link },
        })) : [];
      } catch (e) {
        notify.showError(this.$t("settings.sidebarLinkDefaultsLoadFailed"));
        console.error(e);
      } finally {
        this.loading = false;
      }
    },
    async save() {
      if (this.saving) {
        return;
      }
      this.saving = true;
      try {
        const data = await patchSidebarLinkDefaults({ items: this.items });
        this.items = Array.isArray(data.items) ? data.items.map((item) => ({
          enabled: !!item.enabled,
          enforced: !!item.enforced,
          link: { ...item.link },
        })) : [];
      } catch (e) {
        notify.showError(this.$t("settings.sidebarLinkDefaultsSaveFailed"));
        console.error(e);
      } finally {
        this.saving = false;
      }
    },
    setFlag(index, field, value) {
      if (!this.items[index]) {
        return;
      }
      this.items[index][field] = value;
      void this.save();
    },
    removeItem(index) {
      this.items.splice(index, 1);
      void this.save();
    },
    addCustomLink() {
      const target = this.newCustom.target.startsWith("http")
        ? this.newCustom.target
        : (this.newCustom.target.startsWith("/") ? this.newCustom.target : `/${this.newCustom.target}`);
      this.items.push({
        enabled: true,
        enforced: false,
        link: {
          name: this.newCustom.name,
          category: "custom",
          target,
          icon: "link",
        },
      });
      this.newCustom = { name: "", target: "" };
      this.showAddCustom = false;
      void this.save();
    },
    openYamlEditor() {
      this.yamlText = yaml.dump(this.items, { lineWidth: 120, noRefs: true });
      this.yamlMode = true;
    },
    async applyYaml(text) {
      try {
        const parsed = yaml.load(text);
        if (!Array.isArray(parsed)) {
          throw new Error("expected array");
        }
        this.items = parsed.map((item) => ({
          enabled: !!item.enabled,
          enforced: !!item.enforced,
          link: { ...item.link },
        }));
        this.yamlMode = false;
        await this.save();
        notify.showSuccessToast(this.$t("settings.sidebarLinkDefaultsSaved"));
      } catch (e) {
        notify.showError(this.$t("settings.sidebarLinkDefaultsYamlInvalid"));
        console.error(e);
      }
    },
  },
};
</script>

<style scoped>
.defaults-description {
  margin-top: 0;
  color: var(--textSecondary);
}

.defaults-list {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.default-item {
  display: flex;
  align-items: center;
  gap: 0.75em;
  flex-wrap: wrap;
  background: var(--surfaceSecondary);
}

.default-item-main {
  display: flex;
  align-items: center;
  gap: 0.5em;
  flex: 1 1 12em;
  min-width: 0;
}

.default-item-icon {
  color: var(--primaryColor);
}

.default-item-details {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.default-item-name {
  font-weight: 500;
}

.default-item-meta {
  font-size: 0.85em;
  color: var(--textSecondary);
}

.default-item-toggles {
  display: flex;
  gap: 1em;
  flex-wrap: wrap;
}

.default-toggle {
  display: flex;
  align-items: center;
  gap: 0.35em;
  font-size: 0.9em;
}

.defaults-actions {
  display: flex;
  gap: 0.5em;
  flex-wrap: wrap;
  margin-top: 0.75em;
}

.add-custom-form {
  margin-top: 0.75em;
}

.add-custom-buttons {
  display: flex;
  gap: 0.5em;
  margin-top: 0.5em;
}
</style>
