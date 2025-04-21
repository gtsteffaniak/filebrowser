<template>
  <div id="breadcrumbs" :class="{ 'add-padding': addPadding }">
    <ul>
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
import { extractSourceFromPath } from "@/utils/url.js";

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
  },
  computed: {
    hasUpdate() {
      return state.req.hasUpdate;
    },
    addPadding() {
      return getters.isStickySidebar() || getters.currentView() == "share";
    },
    isCardView() {
      return getters.isCardView();
    },
    items() {
      // double encode # to fix issue with # in path
      // replace all # with %23
      const req = state.req;
      let path = ""
      if (req.path !== undefined) {
        path = state.req.path.replace(/#/g, "%23");
      }
      console.log("path", path);
      let parts = path.split("/");
      if (parts[0] === "") {
        parts.shift();
      }
      if (parts[parts.length - 1] === "") {
        parts.pop();
      }
      let breadcrumbs = [];
      let buildRef = this.base;
      parts.forEach((element) => {
        buildRef = buildRef + encodeURIComponent(element) + "/";
        breadcrumbs.push({
          name: element,
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

  },
  methods: {
    updatePaths() {
      const result = extractSourceFromPath(getters.routePath());
      if (getters.currentView() == "share") {
        this.base = getters.sharePathBase();
        this.path = getters.routePath(this.base);
      } else {
        this.path = decodeURIComponent(result.path);
        this.base = "/";
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

<style scoped>
#breadcrumbs {
  overflow-y: hidden;
}
#breadcrumbs * {
  box-sizing: unset;
}

#breadcrumbs ul {
  display: flex;
  margin: 0;
  margin-bottom: 0.5em;
  margin-top: 0.5em;
  padding: 0;
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
  color: var(--textPrimary);
}

#breadcrumbs ul li a:hover::after {
  border-left-color: var(--primaryColor);
}

#breadcrumbs ul li:last-child a.changeAvailable {
  filter: contrast(0.8) hue-rotate(200deg) saturate(1);
}

</style>

