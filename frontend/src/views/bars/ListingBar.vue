<template>
  <header>
    <action
      class="menu-button"
      icon="close_back"
      :label="$t('buttons.toggleSidebar')"
      @action="toggleSidebar()"
      :disabled="isSearchActive"
    />
    <search v-if="showSearch"></search>
    <action
      class="menu-button"
      icon="grid_view"
      :label="$t('buttons.switchView')"
      @action="switchView"
      :disabled="isSearchActive"
    />
  </header>
</template>

<style>
.flexbar {
  display: flex;
  flex-direction: block;
  justify-content: space-between;
}
</style>
<script>
import { state, mutations, getters } from "@/store";
import Action from "@/components/Action.vue";
import Search from "@/components/Search.vue";

export default {
  name: "listingView",
  components: {
    Action,
    Search,
  },
  data: function () {
    return {
      width: window.innerWidth,
      viewModes: ["list", "compact", "normal", "gallery"],
    };
  },
  computed: {
    showSearch() {
      return getters.isLoggedIn() && getters.currentView() == "listingView";
    },
    isSearchActive() {
      return state.isSearchActive;
    },
    viewIcon() {
      const icons = {
        list: "view_module",
        compact: "view_module",
        normal: "grid_view",
        gallery: "view_list",
      };
      return icons[state.user.viewMode];
    },
  },
  mounted() {
    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("scroll", this.scrollEvent);
    window.addEventListener("resize", this.windowsResize);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("scroll", this.scrollEvent);
    window.removeEventListener("resize", this.windowsResize);
  },
  methods: {
    action() {
      if (this.show) {
        // Assuming `showHover` is a method on a component
        this.$emit("action");
      }
    },
    toggleSidebar() {
      mutations.toggleSidebar();
    },
    async switchView() {
      mutations.closeHovers();
      const currentIndex = this.viewModes.indexOf(state.user.viewMode);
      const nextIndex = (currentIndex + 1) % this.viewModes.length;
      const newView = this.viewModes[nextIndex];
      mutations.updateCurrentUser({ viewMode: newView });
    },
  },
};
</script>
