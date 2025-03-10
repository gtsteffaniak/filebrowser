<template>
  <div class="breadcrumbs">
    <component
      :is="element"
      :to="homePath"
      :aria-label="$t('files.home')"
      :title="$t('files.home')"
    >
      <i class="material-icons">home</i>
    </component>

    <span
      :aria-label="'breadcrumb-link-' + link.name"
      v-for="(link, index) in items"
      :key="index"
    >
      <span class="chevron"><i class="material-icons">keyboard_arrow_right</i></span>
      <component :is="element" :to="link.url">{{ link.name }}</component>
    </span>
    <action style="display: contents" v-if="showShare" icon="share" show="share" />
    <div v-if="isCardView">
      Size:
      <input
        v-model="gallerySize"
        type="range"
        id="gallery-size"
        name="gallery-size"
        min="1"
        max="8"
        @input="updateGallerySize"
        @change="commitGallerySize"
      />
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { extractSourceFromPath, removePrefix, removeLeadingSlash } from "@/utils/url.js";
import Action from "@/components/Action.vue";

export default {
  name: "breadcrumbs",
  components: {
    Action,
  },
  data() {
    return {
      gallerySize: state.user.gallerySize,
      homePath: "/files/",
      path: "",
    };
  },
  props: ["base", "noLink"],
  mounted() {
    this.updatePaths();
  },
  watch: {
    $route() {
      this.updatePaths();
    },
  },
  computed: {
    isCardView() {
      return getters.isCardView();
    },
    items() {
      let parts = removeLeadingSlash(this.path).split("/");
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
      return (
        state.user?.perm &&
        state.user?.perm.share &&
        state.user.username != "publicUser" &&
        getters.currentView() != "share"
      );
    },
  },
  methods: {
    updatePaths() {
      const result = extractSourceFromPath(getters.routePath());
      this.path = result.path;
      if (getters.currentView() == "share") {
        this.homePath = getters.sharePathBase();
        this.path = removePrefix(getters.routePath(), this.homePath + "/");
      }
      if (state.sources.count > 1) {
        this.homePath = "/files/" + result.source;
      }
    },
    updateGallerySize(event) {
      this.gallerySize = parseInt(event.target.value, 10);
    },
    commitGallerySize() {
      mutations.setGallerySize(this.gallerySize);
    },
  },
};
</script>
