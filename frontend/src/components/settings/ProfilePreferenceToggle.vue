<template>
  <ToggleSwitch
    class="item"
    :enforceable="profilePrefs.enforceable"
    :enforced="profilePrefs.enforcedFlag(section, field)"
    :model-value="profilePrefs.sectionBool(section, field)"
    @update:model-value="(v) => profilePrefs.setSectionBool(section, field, v)"
    @change="profilePrefs.emitSectionChange(section, field)"
    @update:enforced="(v) => profilePrefs.emitEnforced(section, field, v)"
    :disabled="effectiveDisabled"
    :enforcement-locked="profilePrefs.isEnforcementLocked(section, field)"
    :name="name"
    :description="effectiveDescription"
  />
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "ProfilePreferenceToggle",
  components: { ToggleSwitch },
  inject: ["profilePrefs"],
  props: {
    section: { type: String, required: true },
    field: { type: String, required: true },
    name: { type: String, required: true },
    description: { type: String, default: "" },
  },
  computed: {
    isLocked() {
      return this.profilePrefs.fieldLocked(this.section, this.field);
    },
    effectiveDisabled() {
      return this.profilePrefs.fieldDisabled(this.section, this.field);
    },
    effectiveDescription() {
      return this.profilePrefs.helpText(this.section, this.field, this.description);
    },
  },
};
</script>
