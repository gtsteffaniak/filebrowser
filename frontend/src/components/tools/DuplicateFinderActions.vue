<template>
  <div v-if="selectedCount > 0" class="duplicate-finder-actions">
    <button @click="handleDelete" class="button delete-button" :disabled="deleting">
      <i v-if="deleting" class="material-icons spin">autorenew</i>
      <i v-else class="material-icons">delete</i>
      <span>{{ $t('general.delete') }} {{ selectedCount }}</span>
    </button>
    <button @click="handleClear" class="button">
      <span>{{ $t('general.clear', { suffix: '' }) }} {{ $t('general.select', { suffix: '' }) }}</span>
    </button>
  </div>
</template>

<script>
export default {
  name: "DuplicateFinderActions",
  props: {
    selectedCount: {
      type: Number,
      default: 0,
    },
    deleting: {
      type: Boolean,
      default: false,
    },
  },
  methods: {
    handleDelete() {
      this.$emit('delete');
    },
    handleClear() {
      this.$emit('clear');
    },
  },
};
</script>

<style scoped>
.duplicate-finder-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
  width: 100%;
  box-sizing: border-box;
}

.button {
  margin: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.button .material-icons {
  font-size: 1.2rem;
}

.delete-button {
  background: #f5576c;
  color: white;
}

.delete-button:hover:not(:disabled) {
  background: #e0455a;
}

.delete-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
