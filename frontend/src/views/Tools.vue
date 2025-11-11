<template>
  <div class="tools-wrapper">
    <!-- Show tools list when on the main /tools route -->
    <div v-if="showToolsList" class="tools-list-container">
      <div class="tools-header">
        <h1>{{ $t('tools.description') }}</h1>
        <p class="description">{{ $t('tools.description') }}</p>
      </div>

      <div id="listingView" class="listing-view normal">
        <router-link
          v-for="tool in tools"
          :key="tool.path"
          :to="tool.path"
          class="item listing-item clickable"
        >
          <div class="tool-icon">
            <i class="material-icons">{{ tool.icon }}</i>
          </div>
          <div class="tool-content">
            <h3 style="margin:0; padding:0;">{{ $t(tool.name) }}</h3>
            <p>{{ $t(tool.description) }}</p>
          </div>
        </router-link>
      </div>
    </div>

    <!-- Render specific tool components -->
    <router-view v-else />
  </div>
</template>

<script>
import { tools } from "@/utils/constants";

export default {
  name: "Tools",
  computed: {
    tools() {
      // Call the tools function to get the array
      return tools();
    },
    showToolsList() {
      // Show the tools list only when on the main /tools route
      return this.$route.name === "Tools";
    },
  },
};
</script>

<style scoped>
.listing-item {
  padding: 1em !important;
}

.tools-wrapper {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.tools-list-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: unset !important;
}

.tool-content {
  padding: 0.5em;
}
</style>
