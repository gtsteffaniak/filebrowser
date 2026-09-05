<template>
  <div class="card-content">
    <p>{{ $t('prompts.newFileTemplateMessage') }}</p>
    <div
      v-for="(item, index) in localItems"
      :key="index"
      :ref="el => dragReorder.itemRefs[index] = el"
      class="new-file-template-row"
      :class="{ 'is-dragging': dragReorder.draggingIndex === index }"
    >
      <i
        class="material-symbols drag-handle"
        aria-hidden="true"
        @pointerdown="dragReorder.onPointerDown(index, $event)"
        @pointermove="dragReorder.onPointerMove($event)"
        @pointerup="dragReorder.onPointerUp($event)"
        @pointercancel="dragReorder.onPointerCancel($event)"
        @lostpointercapture="dragReorder.onPointerCancel($event)"
      >drag_indicator</i>
      <i
        class="file-type-icon"
        :class="typeInfoFor(item).classes"
        aria-hidden="true"
      >{{ typeInfoFor(item).materialSymbol }}</i>
      <div class="form-flex-group form-grow">
        <input
          class="input form-form flat-right"
          type="text"
          v-model.trim="localItems[index]"
          :aria-label="`Template ${index + 1}`"
        />
        <button
          type="button"
          class="button form-button flat-left"
          :title="$t('general.delete')"
          :aria-label="$t('general.delete')"
          @click="removeItem(index)"
        >
          <i class="material-symbols-outlined">delete</i>
        </button>
      </div>
    </div>
    <div class="new-file-template-add">
      <i
        class="file-type-icon"
        :class="typeInfoFor(newItem).classes"
        aria-hidden="true"
      >{{ typeInfoFor(newItem).materialSymbol }}</i>
      <div class="form-flex-group form-grow">
        <input
          class="input form-form flat-right"
          type="text"
          v-model.trim="newItem"
          :placeholder="$t('prompts.newFileTemplatePlaceholder')"
          @keydown.enter.prevent="addItem"
        />
        <button
          type="button"
          class="button form-button flat-left"
          :title="$t('general.add')"
          :aria-label="$t('general.add')"
          @click="addItem"
        >
          <i class="material-symbols-outlined">add</i>
        </button>
      </div>
    </div>
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="cancel"
      :title="$t('general.cancel')"
      :aria-label="$t('general.cancel')"
    >
      {{ $t('general.cancel') }}
    </button>
    <button
      type="button"
      class="button button--flat"
      @click="save"
      :title="$t('general.save')"
      :aria-label="$t('general.save')"
    >
      {{ $t('general.save') }}
    </button>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";
import { getTypeInfoFromExt } from "@/utils/mimetype";
import { createDragReorder } from "@/utils/dragAndDropReorder.js";

export default {
  name: "new-file-template",
  props: {
    items: {
      type: Array,
      default: () => [],
    },
  },
  data() {
    return {
      localItems: Array.isArray(this.items) ? [...this.items] : [],
      newItem: "",
      dragReorder: createDragReorder({
        getOrder: () => [...this.localItems],
        setOrder: (snapshot) => { this.localItems = snapshot; },
      }),
    };
  },
  beforeUnmount() {
    this.dragReorder.cancel();
  },
  methods: {
    addItem() {
      const value = this.newItem.trim();
      if (!value) return;
      this.localItems.push(value);
      this.newItem = "";
    },
    removeItem(index) {
      this.localItems.splice(index, 1);
    },
    typeInfoFor(item) {
      return getTypeInfoFromExt(item);
    },
    cancel() {
      mutations.closeTopPrompt();
    },
    save() {
      const confirm = getters.currentPrompt()?.confirm;
      const cleaned = this.localItems.map((v) => v.trim()).filter(Boolean);
      if (typeof confirm === "function") {
        confirm(cleaned);
      }
      mutations.closeTopPrompt();
    },
  },
};
</script>

<style scoped>
.new-file-template-row,
.new-file-template-add {
  display: flex;
  align-items: center;
  gap: 0.25em;
  margin-bottom: 0.5em;
}

.new-file-template-row {
  border-top: 2px solid transparent;
  border-bottom: 2px solid transparent;
  transition: opacity 0.15s, border-color 0.15s;
}

.new-file-template-row.is-dragging {
  opacity: 0.4;
}

.drag-handle {
  cursor: grab;
  color: var(--textSecondary);
  flex-shrink: 0;
  touch-action: none;
}

.drag-handle:active {
  cursor: grabbing;
}

.file-type-icon {
  flex-shrink: 0;
}

.new-file-template-row .material-symbols-outlined {
  transition: color 0.15s;
}

.new-file-template-row button:hover .material-symbols-outlined {
  font-variation-settings: 'FILL' 1;
}
</style>
