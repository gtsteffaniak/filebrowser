<template>
  <header>
    <action
      class="menu-button"
      icon="menu"
      :label="$t('buttons.toggleSidebar')"
      @action="toggleSidebar()"
    />
    <search />
    <action
      class="menu-button"
      icon="grid_view"
      :label="$t('buttons.switchView')"
      @action="switchView"
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
import Action from "@/components/header/Action.vue";
import Search from "@/components/Search.vue";
import { showError } from "@/notify";

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
      if (getters.currentPromptName() === "sidebar") {
        mutations.closeHovers();
      } else {
        mutations.showHover("sidebar");
      }
    },
    async switchView() {
      mutations.closeHovers();
      const currentIndex = this.viewModes.indexOf(state.user.viewMode);
      const nextIndex = (currentIndex + 1) % this.viewModes.length;
      let data = state.user;
      data.viewMode = this.viewModes[nextIndex];
      try {
        users.update(data, ["viewMode"]);
        mutations.setUser(data);
      } catch (e) {
        showError(e);
      }
    },
  },
};
</script>
