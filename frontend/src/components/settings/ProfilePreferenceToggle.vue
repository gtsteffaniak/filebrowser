<template>
  <ToggleSwitch
    class="item"
    :enforceable="profilePrefs.enforceable"
    :enforced="profilePrefs.enforcedFlag(section, field)"
    :model-value="profilePrefs.sectionBool(section, field)"
    @update:model-value="(v) => profilePrefs.setSectionBool(section, field, v)"
    @change="profilePrefs.emitSectionChange(section, field)"
    @update:enforced="(v) => profilePrefs.emitEnforced(section, field, v)"
    :disabled="valueDisabled"
    :enforcement-disabled="enforcementDisabled"
    :enforcement-locked="profilePrefs.isEnforcementLocked(section, field)"
    :value-tooltip="valueTooltip"
    :name="name"
    :description="description"
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
    valueDisabled() {
      return this.profilePrefs.valueDisabled(this.section, this.field);
    },
    enforcementDisabled() {
      return this.profilePrefs.enforcementDisabled(this.section, this.field);
    },
    valueTooltip() {
      if (this.profilePrefs.isConfigLocked(this.section, this.field)) {
        return this.$t("settings.userDefaultFieldLockedFromConfig");
      }
      return "";
    },
  },
};
</script>
