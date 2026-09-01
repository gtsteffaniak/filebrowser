<template>
  <div class="card-content">
    <p>{{ $t('prompts.newFileTemplateMessage') }}</p>
    <div
      v-for="(item, index) in localItems"
      :key="index"
      :ref="el => itemRefs[index] = el"
      class="new-file-template-row"
      :class="{ 'is-dragging': draggingIndex === index }"
    >
      <i
        class="material-symbols drag-handle"
        aria-hidden="true"
        @pointerdown="onPointerDown(index, $event)"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerCancel"
        @lostpointercapture="onPointerCancel"
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
      pointerId: null,
      dragClone: null,
      dragOffsetY: 0,
      dragCorrectionX: 0,
      dragCorrectionY: 0,
      scrollParent: null,
      autoScrollRAF: null,
      autoScrollDelta: 0,
      lastClientY: 0,
    };
  },
  beforeUnmount() {
    if (this.draggingIndex !== null || this.autoScrollRAF !== null) {
      this.finishDrag(false);
    }
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
    onPointerDown(index, event) {
      if (event.pointerType === "mouse" && event.button !== 0) return;
      event.preventDefault();
      this.draggingIndex = index;
      this.originalItems = [...this.localItems];
      this.pointerId = event.pointerId;
      this.lastClientY = event.clientY;
      // make all pointermove/up events for this pointer go to this element
      event.target.setPointerCapture(event.pointerId);

      const row = getObjectProperty(this.itemRefs, index);
      if (row) {
        const rect = row.getBoundingClientRect();
        this.dragOffsetY = event.clientY - rect.top;
        const clone = row.cloneNode(true);
        clone.style.position = "fixed";
        clone.style.left = `${rect.left}px`;
        clone.style.top = `${rect.top}px`;
        clone.style.width = `${rect.width}px`;
        clone.style.margin = "0";
        clone.style.pointerEvents = "none";
        clone.style.zIndex = "9999";
        clone.style.boxShadow = "0 4px 12px rgba(0, 0, 0, 0.25)";
        row.parentNode.insertBefore(clone, row.nextSibling);
        const actualRect = clone.getBoundingClientRect();
        this.dragCorrectionX = rect.left - actualRect.left;
        this.dragCorrectionY = rect.top - actualRect.top;
        if (this.dragCorrectionX || this.dragCorrectionY) {
          clone.style.left = `${rect.left + this.dragCorrectionX}px`;
          clone.style.top = `${rect.top + this.dragCorrectionY}px`;
        }
        this.dragClone = clone;
        this.scrollParent = this.getScrollParent(row);
      }
    },
    onPointerMove(event) {
      if (this.draggingIndex === null || event.pointerId !== this.pointerId) return;
      event.preventDefault();
      this.lastClientY = event.clientY;
      if (this.dragClone) {
        const top = event.clientY - this.dragOffsetY + (this.dragCorrectionY || 0);
        this.dragClone.style.top = `${top}px`;
      }
      this.updateHoveredIndex(event.clientY);
      this.updateAutoScroll(event.clientY);
    },
    onPointerUp(event) {
      if (event.pointerId !== this.pointerId) return;
      this.finishDrag(true);
    },
    onPointerCancel(event) {
      if (event.pointerId !== this.pointerId) return;
      this.finishDrag(false);
    },
    finishDrag(commit) {
      // If drag was cancelled (not a clean pointerup), restore original order
      if (!commit && this.originalItems !== null) {
        this.localItems = this.originalItems;
      }
      this.originalItems = null;
      this.draggingIndex = null;
      this.pointerId = null;

      if (this.dragClone) {
        this.dragClone.remove();
        this.dragClone = null;
      }
      if (this.autoScrollRAF) {
        cancelAnimationFrame(this.autoScrollRAF);
        this.autoScrollRAF = null;
      }
      this.autoScrollDelta = 0;
      this.scrollParent = null;
    },
    updateHoveredIndex(clientY) {
      if (this.draggingIndex === null) return;
      const entries = Object.entries(this.itemRefs)
        .map(([i, el]) => [Number(i), el])
        .filter(([, el]) => !!el)
        .sort((a, b) => a[0] - b[0]);

      let targetIndex = entries.length ? entries[entries.length - 1][0] : this.draggingIndex;
      for (const [i, el] of entries) {
        const rect = el.getBoundingClientRect();
        if (clientY < rect.top + rect.height / 2) {
          targetIndex = i;
          break;
        }
      }
      if (targetIndex !== this.draggingIndex) {
        const array = [...this.localItems];
        const [moved] = array.splice(this.draggingIndex, 1);
        array.splice(targetIndex, 0, moved);
        this.localItems = array;
        this.draggingIndex = targetIndex;
      }
    },
    getScrollParent(node) {
      let el = node?.parentElement;
      while (el) {
        const style = getComputedStyle(el);
        const scrollable =
          (style.overflowY === "auto" || style.overflowY === "scroll") &&
          el.scrollHeight > el.clientHeight;
        if (scrollable) return el;
        el = el.parentElement;
      }
      return document.scrollingElement || document.documentElement;
    },
    updateAutoScroll(clientY) {
      const EDGE = 50;
      const MAX_SPEED = 16;
      const parent = this.scrollParent;
      if (!parent) return;
      const isWindowScroll = parent === document.scrollingElement || parent === document.documentElement;
      const rect = isWindowScroll ? null : parent.getBoundingClientRect();
      const top = isWindowScroll ? 0 : rect.top;
      const bottom = isWindowScroll ? window.innerHeight : rect.bottom;

      let delta = 0;
      if (clientY < top + EDGE) {
        delta = -MAX_SPEED * (1 - Math.max(0, clientY - top) / EDGE);
      } else if (clientY > bottom - EDGE) {
        delta = MAX_SPEED * (1 - Math.max(0, bottom - clientY) / EDGE);
      }
      this.autoScrollDelta = delta;
      if (delta !== 0 && this.autoScrollRAF === null) {
        this.autoScroll();
      }
    },
    autoScroll() {
      const isWindowScroll =
        this.scrollParent === document.scrollingElement || this.scrollParent === document.documentElement;
      const step = () => {
        if (this.autoScrollDelta !== 0 && this.scrollParent) {
          if (isWindowScroll) {
            window.scrollBy(0, this.autoScrollDelta);
          } else {
            this.scrollParent.scrollTop += this.autoScrollDelta;
          }
          this.updateHoveredIndex(this.lastClientY);
          this.autoScrollRAF = requestAnimationFrame(step);
        } else {
          this.autoScrollRAF = null;
        }
      };
      this.autoScrollRAF = requestAnimationFrame(step);
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
