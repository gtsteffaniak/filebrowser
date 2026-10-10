<template>
  <EntityPickerButton
    kind="group"
    multiple
    :label="$t('access.userGroups')"
    :description="description"
    :username="username"
    :disabled="disabled"
    :initial-selected="modelValue"
    @select="update"
  />
</template>

<script>
import EntityPickerButton from "@/components/settings/EntityPickerButton.vue";

export default {
  name: "UserGroupsField",
  components: { EntityPickerButton },
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
    /** The user whose group memberships are being edited. */
    username: {
      type: String,
      default: "",
    },
    /** Optional help tooltip next to the row label. */
    description: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  methods: {
    update(names) {
      this.$emit("update:modelValue", names);
    },
  },
};
</script>
