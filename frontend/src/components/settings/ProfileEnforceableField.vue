<template>
  <div
    class="profile-enforceable-field"
    :class="enforceable ? 'toggle-container toggle-container--enforceable item' : 'preference-field-block'"
  >
    <template v-if="enforceable">
      <div
        class="toggle-row toggle-row--value border-radius"
        :class="{ 'profile-enforceable-field__value--stacked': stacked }"
      >
        <slot />
      </div>
      <slot name="enforce" />
    </template>
    <div v-else class="profile-enforceable-field__value">
      <slot />
    </div>
  </div>
</template>

<script>
export default {
  name: "ProfileEnforceableField",
  props: {
    enforceable: {
      type: Boolean,
      default: false,
    },
    /** Multi-line value block (heading + control). False for SettingsButton value rows. */
    stacked: {
      type: Boolean,
      default: true,
    },
  },
};
</script>

<style scoped>
.profile-enforceable-field__value {
  width: 100%;
  box-sizing: border-box;
}

.profile-enforceable-field.toggle-container--enforceable {
  font-size: 1rem;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.toggle-row--value.profile-enforceable-field__value--stacked) {
  flex-direction: column;
  align-items: stretch;
  justify-content: center;
  gap: 0.5em;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.toggle-row--value:not(.profile-enforceable-field__value--stacked) > *) {
  width: 100%;
  min-width: 0;
}

.profile-enforceable-field.toggle-container--enforceable:hover :deep(.profile-enforce-row),
.profile-enforceable-field.toggle-container--enforceable:hover :deep(.profile-enforce-row:hover) {
  background-color: transparent;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.profile-enforce-row) {
  margin-top: 0;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.settings-items) {
  width: 100%;
  margin-top: 0;
  margin-bottom: 0;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.settings-items .item) {
  margin: 0;
  padding: 0;
}

.profile-enforceable-field.toggle-container--enforceable :deep(h3),
.profile-enforceable-field.toggle-container--enforceable :deep(h4) {
  margin: 0;
  width: 100%;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.button-group) {
  margin: 0;
  width: 100%;
  max-width: 100%;
}

.profile-enforceable-field.toggle-container--enforceable :deep(.form-flex-group) {
  width: 100%;
}
</style>
