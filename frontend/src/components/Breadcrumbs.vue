<template>
  <div class="breadcrumbs">
    <router-link :to="base" :aria-label="$t('files.home')" :title="$t('files.home')">
      <i class="material-icons">home</i>
    </router-link>

    <span
      :aria-label="'breadcrumb-link-' + link.name"
      v-for="(link, index) in items"
      :key="index"
    >
      <span class="chevron"><i class="material-icons">keyboard_arrow_right</i></span>
      <router-link :to="link.url">{{ link.name }}</router-link>
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
import { baseURL } from "@/utils/constants.js";
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
      base: "/files/",
      path: "",
    };
  },
  props: ["noLink"],
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
      let buildRef = this.base;
      parts.forEach((element) => {
        buildRef = buildRef + encodeURIComponent(element) + "/";
        breadcrumbs.push({
          name: decodeURIComponent(element),
          url: buildRef,
        });
      });

      if (breadcrumbs.length > 3) {
        while (breadcrumbs.length !== 4) {
          breadcrumbs.shift();
        }
        breadcrumbs[0].name = "...";
      }

      return breadcrumbs;
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
      if (getters.currentView() == "share") {
        this.base = getters.sharePathBase();
        this.path = removePrefix(getters.routePath(), this.base + "/");
        this.path = this.path.split("/").slice(3).join(); // remove first two components /share/sharename
      } else {
        this.path = decodeURIComponent(result.path);
        this.base = baseURL;
        if (state.serverHasMultipleSources) {
          this.base = `${this.base}${result.source}/`;
        }
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
