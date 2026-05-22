<template>
  <div v-if="show" class="sidebar-directories card">
    <div class="section-title">{{ $t('general.folders') }}</div>
    <div class="inner-card">
      <button
        v-for="dir in dirs"
        :key="dir.path"
        class="dir-button"
        :aria-label="dir.name"
        @click="navigate(dir)"
      >
        <i class="material-icons dir-icon">folder</i>
        <span class="dir-name">{{ dir.name }}</span>
      </button>
    </div>
  </div>
</template>

<script>
import { state, getters } from "@/store";
import { buildItemUrl } from "@/utils/url";

export default {
  name: "SidebarDirectories",
  computed: {
    isListingView() {
      return getters.currentView() === "listingView";
    },
    dirs() {
      return getters.reqItems().dirs || [];
    },
    show() {
      return this.isListingView && this.dirs.length > 0 && !state.isSearchActive;
    },
  },
  methods: {
    navigate(dir) {
      const url = buildItemUrl(dir.source || state.req.source, dir.path);
      this.$router.push({ path: url });
    },
  },
};
</script>

<style scoped>
.sidebar-directories {
  padding: 0.63em;
  margin-top: 0.35em;
  background-color: var(--background);
  border-radius: 1em;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.inner-card {
  display: flex;
  flex-direction: column;
  gap: 0.32em;
  width: 100%;
}

.dir-button {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 100%;
  padding: 0.47em 0.63em;
  border-radius: 0.5em;
  background-color: transparent;
  color: var(--textPrimary);
  border: none;
  cursor: pointer;
  transition: background-color 0.2s, transform 0.1s;
  gap: 0.47em;
  text-align: left;
}

.dir-button:hover {
  background-color: var(--alt-background);
  transform: translateY(-2px);
}

.dir-button:active {
  transform: translateY(0);
}

.dir-icon {
  color: white;
  font-size: 1.25em;
  background-color: #50898e;
  padding: 0.3em;
  border-radius: 0.4em;
  flex-shrink: 0;
}

.section-title {
  font-size: 0.7em;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #6b7280;
  padding: 0 0.2em 0.4em;
}

.dir-name {
  color: #6b7280;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
