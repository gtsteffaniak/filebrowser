<template>
  <div
    class="profile-enforceable-field"
    :class="enforceable ? 'toggle-container toggle-container--enforceable item' : 'preference-field-block'"
  >
    <div
      class="profile-enforceable-field__value"
      :class="{ 'toggle-row toggle-row--value border-radius': enforceable }"
    >
      <slot />
    </div>
    <template v-if="enforceable">
      <slot name="enforce" />
    </template>
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
  },
};
</script>

<style scoped>
.profile-enforceable-field__value {
  width: 100%;
  box-sizing: border-box;
}

.profile-enforceable-field.toggle-container--enforceable {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.35em;
  padding: 0.35em;
  border-radius: var(--borderRadius);
  transition: background-color 0.15s ease;
  font-size: 1rem;
}

.profile-enforceable-field.toggle-container--enforceable:hover {
  background-color: var(--surfaceSecondary);
}

.profile-enforceable-field.toggle-container--enforceable .profile-enforceable-field__value {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
  min-height: 3.25em;
  padding: 0.5em 1em;
  justify-content: center;
}

.profile-enforceable-field.toggle-container--enforceable .toggle-row {
  box-sizing: border-box;
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
