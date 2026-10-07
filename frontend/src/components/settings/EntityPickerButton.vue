<template>
  <div v-if="!compact" class="settings-button item toggle-container" :class="{ disabled }">
    <div class="toggle-name-container">
      <span class="toggle-name">{{ resolvedLabel }}</span>
      <HelpTooltipIcon v-if="description" :text="description" />
      <slot />
    </div>
    <button
      type="button"
      class="button button--settings-control entity-picker-button"
      :disabled="disabled"
      :aria-label="resolvedAriaLabel"
      @click="openPicker"
    >
      <i class="material-symbols-outlined">{{ icon }}</i>
    </button>
  </div>
  <button
    v-else
    type="button"
    class="button clickable entity-picker-button entity-picker-button--compact"
    :disabled="disabled"
    :aria-label="resolvedAriaLabel"
    :title="resolvedAriaLabel"
    @click="openPicker"
  >
    <i class="material-symbols-outlined">{{ icon }}</i>
  </button>
</template>

<script>
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";

export default {
  name: "EntityPickerButton",
  components: { HelpTooltipIcon },

  props: {
    /** What to pick: "user" or "group". */
    kind: {
      type: String,
      default: "user",
      validator: (v) => v === "user" || v === "group",
    },
    multiple: {
      type: Boolean,
      default: false,
    },
    /** Names to exclude from the pickable list (e.g. already selected). */
    exclude: {
      type: Array,
      default: () => [],
    },
    /** User context mode: prompt shows this username and its memberships on top. */
    username: {
      type: String,
      default: "",
    },
    /** Names pre-checked on open (e.g. the user's existing groups). */
    initialSelected: {
      type: Array,
      default: () => [],
    },
    /** Row/button text; defaults to a translated label based on kind/multiple. */
    label: {
      type: String,
      default: "",
    },
    ariaLabel: {
      type: String,
      default: "",
    },
    /** Optional help tooltip next to the row label. */
    description: {
      type: String,
      default: "",
    },
    /** Icon inside the picker button. */
    icon: {
      type: String,
      default: "open_in_new",
    },
    /** Render only the icon button (for inline form rows). */
    compact: {
      type: Boolean,
      default: false,
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },

  emits: ["select", "cancel"],

  data() {
    return {
      pendingContextId: null,
    };
  },

  computed: {
    resolvedLabel() {
      if (this.label) {
        return this.label;
      }
      if (this.multiple) {
        return this.kind === "group"
          ? this.$t("access.selectGroups")
          : this.$t("access.selectUsers");
      }
      return this.kind === "group"
        ? this.$t("access.selectGroup")
        : this.$t("access.selectUser");
    },
    resolvedAriaLabel() {
      return this.ariaLabel || this.resolvedLabel;
    },
  },

  mounted() {
    eventBus.on("entitiesSelected", this.onEntitiesSelected);
    eventBus.on("entityPickerCancelled", this.onEntityPickerCancelled);
  },

  beforeUnmount() {
    eventBus.off("entitiesSelected", this.onEntitiesSelected);
    eventBus.off("entityPickerCancelled", this.onEntityPickerCancelled);
  },

  methods: {
    openPicker() {
      this.pendingContextId = `entity-picker-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
      mutations.showPrompt({
        name: "entityPicker",
        props: {
          kind: this.kind,
          multiple: this.multiple,
          exclude: this.exclude,
          username: this.username,
          initialSelected: this.initialSelected,
          selectionContextId: this.pendingContextId,
          title: this.resolvedLabel,
        },
      });
    },
    onEntitiesSelected(data) {
      if (!this.pendingContextId || data?.selectionContextId !== this.pendingContextId) {
        return;
      }
      this.pendingContextId = null;
      this.$emit("select", data.names || []);
    },
    onEntityPickerCancelled(data) {
      if (!this.pendingContextId || data?.selectionContextId !== this.pendingContextId) {
        return;
      }
      this.pendingContextId = null;
      this.$emit("cancel");
    },
  },
};
</script>

<style scoped>
.settings-button.toggle-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 1rem;
  width: 100%;
}

.toggle-name-container {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.25em;
}

.entity-picker-button--compact {
  width: auto;
  min-width: 2.2em;
  padding: 0 0.4em;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
</style>
