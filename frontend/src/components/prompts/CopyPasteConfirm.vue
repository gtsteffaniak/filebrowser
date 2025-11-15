<template>
  <div class="card-title">
    <h2>{{ $t("prompts.copyPasteConfirm") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ operation === 'copy' ? $t("prompts.copyItemQuestion") : $t("prompts.moveItemQuestion") }}</p>

    <div class="path-info">
      <div class="path-section">
        <strong>{{ $t("general.path", { suffix: ':' }) }}</strong>
        <div class="path-display">
          <span class="path-item" v-for="(item, index) in sourceItems" :key="index">
            {{ item }}
          </span>
        </div>
      </div>

      <div class="path-section">
        <strong>{{ $t("prompts.destinationPath") }}</strong>
        <div class="path-display">
          <span class="path-item" v-for="(item, index) in destinationItems" :key="index">
            {{ item }}
          </span>
        </div>
      </div>
    </div>
  </div>

  <div class="card-action">
    <button
      @click="closeHovers"
      class="button button--flat button--grey"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button
      @click="confirm"
      class="button button--flat"
      :aria-label="operation === 'copy' ? $t('general.copy') : $t('general.move')"
      :title="operation === 'copy' ? $t('general.copy') : $t('general.move')">
      {{ operation === 'copy' ? $t('general.copy') : $t('general.move') }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";

export default {
  name: "copy-paste-confirm",
  props: {
    operation: {
      type: String,
      required: true,
      validator: (value) => ["copy", "move"].includes(value),
    },
    items: {
      type: Array,
      required: true,
    },
    onConfirm: {
      type: Function,
      required: true,
    },
  },
  computed: {
    sourceItems() {
      return this.items.map(item => {
        const source = item.fromSource || '';
        const path = item.from || '';
        return source ? `${source}${path}` : path;
      });
    },
    destinationItems() {
      return this.items.map(item => {
        const source = item.toSource || '';
        const path = item.to || '';
        return source ? `${source}${path}` : path;
      });
    },
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    confirm() {
      this.onConfirm();
      mutations.setReload(true);
      mutations.closeHovers();
    },
  },
};
</script>

<style scoped>
.path-info {
  margin-top: 1em;
  display: flex;
  flex-direction: column;
  gap: 1em;
}

.path-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.path-display {
  display: flex;
  flex-direction: column;
  gap: 0.25em;
  padding: 0.75em;
  background-color: var(--surfaceSecondary);
  border-radius: 4px;
  max-height: 150px;
  overflow-y: auto;
}

.path-item {
  font-family: monospace;
  font-size: 0.9em;
  word-break: break-all;
  padding: 0.25em 0;
  color: var(--textPrimary);
}
</style>

