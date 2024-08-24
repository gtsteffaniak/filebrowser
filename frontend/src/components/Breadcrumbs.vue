<template>
  <div class="breadcrumbs">
    <component
      :is="element"
      :to="base || ''"
      :aria-label="$t('files.home')"
      :title="$t('files.home')"
    >
      <i class="material-icons">home</i>
    </component>

    <span v-for="(link, index) in items" :key="index">
      <span class="chevron"><i class="material-icons">keyboard_arrow_right</i></span>
      <component :is="element" :to="link.url">{{ link.name }}</component>
    </span>
    <action style="display: contents" v-if="showShare" icon="share" show="share" />
    <div v-if="isResizableView">
      Size:
      <input
        v-model="gallerySize"
        type="range"
        id="gallary-size"
        name="gallary-size"
        :value="gallerySize"
        min="0"
        max="10"
      />
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store"; // Import mutations as well
import Action from "@/components/header/Action.vue";

export default {
  name: "breadcrumbs",
  components: {
    Action,
  },
  data() {
    return {
      gallerySize: state.user.gallerySize,
    };
  },
  watch: {
    gallerySize(newValue) {
      this.gallerySize = parseInt(newValue, 0); // Update the user object
      mutations.setGallerySize(this.gallerySize);
    },
  },
  props: ["base", "noLink"],
  computed: {
    isResizableView() {
      return getters.isResizableView();
    },
    items() {
      const relativePath = state.route.path.replace(this.base, "");
      let parts = relativePath.split("/");

      if (parts[0] === "") {
        parts.shift();
      }

      if (parts[parts.length - 1] === "") {
        parts.pop();
      }

      let breadcrumbs = [];

      for (let i = 0; i < parts.length; i++) {
        if (i === 0) {
          breadcrumbs.push({
            name: decodeURIComponent(parts[i]),
            url: this.base + "/" + parts[i] + "/",
          });
        } else {
          breadcrumbs.push({
            name: decodeURIComponent(parts[i]),
            url: breadcrumbs[i - 1].url + parts[i] + "/",
          });
        }
      }

      if (breadcrumbs.length > 3) {
        while (breadcrumbs.length !== 4) {
          breadcrumbs.shift();
        }

        breadcrumbs[0].name = "...";
      }

      return breadcrumbs;
    },
    element() {
      if (this.noLink !== undefined) {
        return "span";
      }

      return "router-link";
    },
    showShare() {
      // Ensure user properties are accessed safely
      if (state.route.path.startsWith("/share")) {
        return false;
      }
      return state.user?.perm && state.user?.perm.share; // Access from state directly
    },
  },
  methods: {
    // Example of a method using mutations
    updateUserPermissions(newPerms) {
      mutations.updateUser({ perm: newPerms });
    },
  },
};
</script>
