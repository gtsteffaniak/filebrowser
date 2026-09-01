<template>
  <div class="card-content">
    <p>{{ $t('prompts.newFileTemplateMessage') }}</p>
    <div
      v-for="(item, index) in localItems"
      :key="index"
      :ref="el => itemRefs[index] = el"
      class="new-file-template-row"
      :class="{ 'is-dragging': draggingIndex === index }"
      @dragover.prevent="onDragOver(index)"
      @drop.prevent="onDrop(index)"
    >
      <i
        class="material-symbols drag-handle"
        aria-hidden="true"
        draggable="true"
        @dragstart="onDragStart(index, $event)"
        @dragend="onDragEnd"
      >drag_indicator</i>
      <i
        class="file-type-icon"
        :class="typeInfoFor(item).classes"
        aria-hidden="true"
      >{{ typeInfoFor(item).materialSymbol }}</i>
      <input
        class="input"
        type="text"
        v-model.trim="localItems[index]"
        :aria-label="`Template ${index + 1}`"
      />
      <button
        type="button"
        class="button button--flat button--grey"
        :title="$t('general.delete')"
        :aria-label="$t('general.delete')"
        @click="removeItem(index)"
      >
        <i class="material-symbols-outlined">delete</i>
      </button>
    </div>
    <div class="new-file-template-add">
      <i
        class="file-type-icon"
        :class="typeInfoFor(newItem).classes"
        aria-hidden="true"
      >{{ typeInfoFor(newItem).materialSymbol }}</i>
      <input
        class="input"
        type="text"
        v-model.trim="newItem"
        :placeholder="$t('prompts.newFileTemplatePlaceholder')"
        @keyup.enter="addItem"
      />
      <button
        type="button"
        class="button button--flat"
        :title="$t('general.add')"
        :aria-label="$t('general.add')"
        @click="addItem"
      >
        {{ $t('general.add') }}
      </button>
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
import { getObjectProperty } from '@/utils/object.js';
import { getTypeInfoFromExt } from "@/utils/mimetype";

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
      draggingIndex: null,
      itemRefs: {},
      originalItems: null, // store original order in case drag gets cancelled
    };
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
    // I tried to make the same drag behavior that the SidebarLinks.vue prompt has 
    onDragStart(index, event) {
      this.draggingIndex = index;
      this.originalItems = [...this.localItems];

      if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", String(index));

        const row = getObjectProperty(this.itemRefs, index);
        if (row) {
          const clone = row.cloneNode(true);
          clone.style.position = 'fixed';
          clone.style.top = '-9999px';
          clone.style.left = '-9999px';
          clone.style.width = `${row.offsetWidth}px`;

          // Insert it as a sibling so inherits the theme dark mode
          row.parentNode.insertBefore(clone, row.nextSibling);
          event.dataTransfer.setDragImage(clone, event.offsetX, event.offsetY);

          setTimeout(() => {
            clone.remove();
          }, 0);
        }
      }
    },
    onDragOver(index) {
      if (this.draggingIndex === null || this.draggingIndex === index) return;
      const array = [...this.localItems];
      const [moved] = array.splice(this.draggingIndex, 1);
      array.splice(index, 0, moved);

      this.localItems = array;
      this.draggingIndex = index; // Update positio
    },
    onDrop() {
      this.draggingIndex = null;
      this.originalItems = null; // Clear the backup
    },
    onDragEnd() {
      // If drag was cancelled restore original order
      if (this.originalItems !== null) {
        this.localItems = this.originalItems;
        this.originalItems = null;
      }
      this.draggingIndex = null;
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
}

.drag-handle:active {
  cursor: grabbing;
}

.file-type-icon {
  flex-shrink: 0;
}

.new-file-template-row .input,
.new-file-template-add .input {
  flex: 1;
}

.new-file-template-row .material-symbols-outlined {
  transition: color 0.15s;
}

.new-file-template-row button:hover .material-symbols-outlined {
  color: red;
  font-variation-settings: 'FILL' 1;
}

</style>
