<template>
  <div class="card-content">
    <div v-if="username" class="entity-picker-user">
      <i class="material-symbols-outlined">person</i>
      <span class="entity-picker-username">{{ username }}</span>
    </div>
    <EntityPickerList
      ref="entityList"
      :kind="kind"
      :multiple="multiple"
      :exclude="exclude"
      v-model="picked"
      @enter="confirmSelection"
    />
  </div>

  <div class="card-actions">
    <button
      v-if="kind === 'group'"
      type="button"
      class="button button--flat entity-picker-create"
      :aria-label="$t('access.newGroup')"
      :title="$t('access.newGroup')"
      @click="startCreate"
    >
      <i class="material-symbols">add</i>
    </button>
    <button
      type="button"
      class="button button--flat"
      :disabled="picked.length === 0 && !username"
      @click="confirmSelection"
      :aria-label="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import EntityPickerList from "./EntityPickerList.vue";

export default {
  name: "entity-picker",
  components: { EntityPickerList },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
    /** What to list and pick. */
    kind: {
      type: String,
      default: "user",
      validator: (v) => v === "user" || v === "group",
    },
    /** Already selected names; shown pre-checked and excluded from the list. */
    exclude: {
      type: Array,
      default: () => [],
    },
    multiple: {
      type: Boolean,
      default: false,
    },
    /** User context mode: shows the username and its current memberships on top. */
    username: {
      type: String,
      default: "",
    },
    /** Names pre-checked on open (e.g. the user's existing groups). */
    initialSelected: {
      type: Array,
      default: () => [],
    },
    /** When set, included on entitiesSelected so listeners can tell this pick apart from others. */
    selectionContextId: {
      type: String,
      default: null,
    },
  },
  data() {
    return {
      picked: [...this.initialSelected],
      selectionFinished: false,
    };
  },
  beforeUnmount() {
    if (this.selectionContextId && !this.selectionFinished) {
      eventBus.emit("entityPickerCancelled", {
        selectionContextId: this.selectionContextId,
      });
    }
  },
  methods: {
    startCreate() {
      this.$refs.entityList?.startCreate();
    },
    confirmSelection() {
      if (this.picked.length === 0 && !this.username) {
        return;
      }
      this.selectionFinished = true;
      const payload = { names: [...this.picked], kind: this.kind };
      if (this.selectionContextId) {
        payload.selectionContextId = this.selectionContextId;
      }
      eventBus.emit("entitiesSelected", payload);
      mutations.closeTopPrompt(this.promptId ?? undefined);
    },
  },
};
</script>

<style scoped>
.entity-picker-create {
  margin-right: auto;
}

.entity-picker-user {
  display: flex;
  align-items: center;
  gap: 0.4em;
  margin-bottom: 0.5em;
  font-weight: 600;
}

.entity-picker-username {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
