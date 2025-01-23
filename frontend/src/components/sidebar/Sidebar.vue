<template>
  <nav
    id="sidebar"
    :class="{ active: active, 'dark-mode': isDarkMode, 'behind-overlay': behindOverlay }"
  >
    <SidebarSettings v-if="isSettings"></SidebarSettings>
    <SidebarGeneral v-else-if="isLoggedIn"></SidebarGeneral>

    <div class="buffer"></div>
    <div class="credits">
      <span v-for="item in externalLinks" :key="item.title">
        <a :href="item.url" target="_blank" :title="item.title">{{ item.text }}</a>
      </span>
      <span v-if="name != ''"
        ><h4>{{ name }}</h4></span
      >
    </div>
  </nav>
</template>

<script>
import { externalLinks, name } from "@/utils/constants";
import { getters, mutations, state } from "@/store"; // Import your custom store
import SidebarGeneral from "./General.vue";
import SidebarSettings from "./Settings.vue";

export default {
  name: "sidebar",
  components: {
    SidebarGeneral,
    SidebarSettings,
  },
  data() {
    return {
      externalLinks,
      name,
    };
  },
  computed: {
    isDarkMode: () => getters.isDarkMode(),
    isLoggedIn: () => getters.isLoggedIn(),
    isSettings: () => getters.isSettings(),
    active: () => getters.isSidebarVisible(),
    behindOverlay: () => state.isSearchActive,
  },
  methods: {
    // Show the help overlay
    help() {
      mutations.showHover("help");
    },
  },
};
</script>

<style>
.sidebar-scroll-list {
  overflow: auto;
  margin-bottom: 0px !important;
}

#sidebar {
  display: flex;
  flex-direction: column;
  padding: 1em;
  width: 20em;
  position: fixed;
  z-index: 4;
  left: -20em;
  height: 100%;
  box-shadow: 0 0 5px rgba(0, 0, 0, 0.1);
  transition: 0.5s ease;
  top: 4em;
  padding-bottom: 4em;
  background-color: #dddddd;
}

#sidebar.behind-overlay {
  z-index: 3;
}

#sidebar.sticky {
  z-index: 3;
}

@supports (backdrop-filter: none) {
  nav {
    backdrop-filter: blur(16px) invert(0.1);
  }
}

body.rtl nav {
  left: unset;
  right: -17em;
}

#sidebar.active {
  left: 0;
}

#sidebar.rtl nav.active {
  left: unset;
  right: 0;
}

#sidebar .action {
  width: 100%;
  display: block;
  white-space: nowrap;
  height: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}

body.rtl .action {
  direction: rtl;
  text-align: right;
}

#sidebar .action > * {
  vertical-align: middle;
}

/* * * * * * * * * * * * * * * *
 *            FOOTER           *
 * * * * * * * * * * * * * * * */

.credits {
  font-size: 1em;
  color: var(--textSecondary);
  padding-left: 1em;
  padding-bottom: 1em;
}

.credits > span {
  display: block;
  margin-top: 0.5em;
  margin-left: 0;
}

.credits a,
.credits a:hover {
  color: inherit;
  cursor: pointer;
}

.buffer {
  flex-grow: 1;
}

.card-wrapper {
  display: flex !important;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 1em !important;
  min-height: 4em;
  box-shadow: 0 2px 2px #00000024, 0 1px 5px #0000001f, 0 3px 1px -2px #0003;
  /* overflow: auto; */
  border-radius: 1em;
  height: 100%;
}

.clickable {
  cursor: pointer;
}

.clickable:hover {
  box-shadow: 0 2px 2px #00000024, 0 1px 5px #0000001f, 0 3px 1px -2px #0003;
}
</style>
