<template>
  <div
    v-for="setting in settings"
    :key="setting.id + '-sidebar'"
    :id="setting.id + '-sidebar'"
    class="card clickable"
    @click="setView(setting.id + '-main')"
    :class="{ 'active-settings': active(setting.id + '-main') }"
  >
    <div class="card-wrapper">{{ setting.label }}</div>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { settings } from "@/utils/constants";

export default {
  name: "SidebarSettings",
  data() {
    return {
      settings // Initialize the settings array in data
    };
  },
  computed: {
    currentHash: () => getters.currentHash(),
  },
  methods: {
    active: (view) => state.activeSettingsView === view,
    setView(view) {
      mutations.setActiveSettingsView(view);
    },
  },
};
</script>
<style>
.active-settings {
  font-weight: bold;
  /* border-color: white; */
  border-style: solid;
}
</style>
