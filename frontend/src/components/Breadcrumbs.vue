<template>
  <div id="breadcrumbs" :class="{ 'add-padding': addPadding }">
    <ul v-if="items.length > 0">
      <li>
        <router-link :to="base" :aria-label="$t('files.home')" :title="$t('files.home')">
          <i class="material-icons">home</i>
        </router-link>
      </li>
      <li class="item" v-for="(link, index) in items" :key="index">
        <router-link
          :to="link.url"
          :aria-label="'breadcrumb-link-' + link.name"
          :title="link.name"
          :key="index"
          :class="{ changeAvailable: hasUpdate }"
        >
          {{ link.name }}
        </router-link>
      </li>
    </ul>
    <div v-if="showGallerySize" class="gallery-size card">
      {{ $t("files.size") }}<span class="sr-only">:</span>  <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
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
import { encodedPath } from "@/utils/url.js";

export default {
  name: "breadcrumbs",
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
    req() {
      this.updatePaths();
    },
  },
  computed: {
    req() {
      return state.req;
    },
    hasUpdate() {
      return state.req.hasUpdate;
    },
    addPadding() {
      return getters.isStickySidebar() || getters.isShare();
    },
    showGallerySize() {
      return getters.isCardView() && state.req?.items?.length > 0;
    },
    items() {
      const req = state.req;
      if (!req.items || !req.path) {
        return [];
      }
      let encodedPathString = encodedPath(state.req.path);
      let originalParts = state.req.path.split("/");
      let encodedParts = encodedPathString.split("/");
      // Remove empty strings from both arrays consistently
      if (originalParts[0] === "") {
        originalParts.shift();
        encodedParts.shift();
      }
      if (originalParts[originalParts.length - 1] === "") {
        originalParts.pop();
        encodedParts.pop();
      }
      let breadcrumbs = [];
      let buildRef = this.base;
      for (let i = 0; i < originalParts.length; i++) {
        const origPart = originalParts[i];
        const encodedElement = encodedParts[i];
        buildRef = buildRef + encodedElement + "/";
        breadcrumbs.push({
          name: origPart,
          url: buildRef,
        });
      }
      if (breadcrumbs.length > 3) {
        while (breadcrumbs.length !== 4) {
          breadcrumbs.shift();
        }
        breadcrumbs[0].name = "...";
      }

      return breadcrumbs;
    },
  },
  methods: {
    updatePaths() {
      if (getters.isShare()) {
        this.base = getters.sharePathBase();
        this.path = getters.routePath(this.base);

      } else {
        this.path = encodedPath(state.req.path);
        if (state.serverHasMultipleSources) {
          this.base = `/files/${state.req.source}/`;
        } else {
          this.base = "/files/";
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

<style scoped>
#breadcrumbs {
  margin-top: 0.5em;
  overflow-y: hidden;
}
#breadcrumbs * {
  box-sizing: unset;
}

#breadcrumbs ul {
  display: flex;
  margin: 0;
  padding: 0;
  margin-bottom: 0.5em;
}

#breadcrumbs ul li {
  display: inline-block;
  margin: 0 10px 0 0;
}

#breadcrumbs ul li a {
  display: flex;
  height: 1em;
  background: var(--alt-background);
  text-align: center;
  padding: 1em;
  padding-left: 2em;
  position: relative;
  text-decoration: none;
  color: var(--textPrimary);
  border-radius: 0;
  align-content: center;
  align-items: center;
}

#breadcrumbs ul li a::after {
  content: "";
  border-top: 1.5em solid transparent;
  border-bottom: 1.5em solid transparent;
  border-left: 1.5em solid var(--alt-background);
  position: absolute;
  right: -1.5em;
  top: 0;
  z-index: 1;
}

#breadcrumbs ul li a::before {
  content: "";
  border-top: 1.5em solid transparent;
  border-bottom: 1.5em solid transparent;
  border-left: 1.5em solid var(--background);
  position: absolute;
  left: 0;
  top: 0;
}

#breadcrumbs ul li:first-child a {
  border-top-left-radius: 1em;
  border-bottom-left-radius: 1em;
  padding-left: 1.5em;
}

#breadcrumbs ul li:first-child a::before {
  display: none;
}

#breadcrumbs ul li:last-child a {
  padding-right: 1.5em;
  border-top-right-radius: 1em;
  border-bottom-right-radius: 1em;
}

#breadcrumbs ul li:last-child a::after {
  display: none;
}

#breadcrumbs ul li a:hover {
  background: var(--primaryColor);
  color: white;
}

#breadcrumbs ul li a:hover::after {
  border-left-color: var(--primaryColor);
}

#breadcrumbs ul li:last-child a.changeAvailable {
  filter: contrast(0.8) hue-rotate(200deg) saturate(1);
}

.gallery-size {
  display: flex;
  width: fit-content;
  padding: 0.5em;
  margin-bottom: 0.5em;
}
input[type="range"] {
  accent-color: var(--primaryColor);
}
</style>
