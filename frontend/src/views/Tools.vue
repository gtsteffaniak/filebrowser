<template>
  <div class="tools-wrapper">
    <!-- Show tools list when no tool is selected -->
    <div v-if="showToolsList" class="tools-list-container">
      <div class="tools-header">
        <h1>{{ $t('tools.title') }}</h1>
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
            <h3 style="margin:0; padding:0;">{{ tool.name }}</h3>
            <p>{{ tool.description }}</p>
          </div>
        </router-link>
      </div>
    </div>

    <!-- Dynamically render the selected tool component -->
    <component v-else-if="currentToolComponent" :is="currentToolComponent" />

    <!-- Show error if tool not found -->
    <div v-else class="tool-not-found">
      <i class="material-icons">error_outline</i>
      <h2>{{ $t('tools.toolNotFound') }}</h2>
      <router-link to="/tools" class="button button--flat">{{ $t('tools.backToTools') }}</router-link>
    </div>
  </div>
</template>

<script>
import { defineAsyncComponent } from 'vue';
import { tools } from "@/utils/constants";

export default {
  name: "Tools",
  computed: {
    tools() {
      return tools();
    },
    toolName() {
      return this.$route.params.toolName;
    },
    showToolsList() {
      return !this.toolName;
    },
    currentToolComponent() {
      if (!this.toolName) return null;

      // Map tool names to their components
      const toolComponents = {
        'sizeViewer': defineAsyncComponent(() => import('@/views/tools/SizeViewer.vue')),
        'duplicateFinder': defineAsyncComponent(() => import('@/views/tools/DuplicateFinder.vue')),
        'materialIconPicker': defineAsyncComponent(() => import('@/views/tools/MaterialIconPicker.vue')),
      };

      return toolComponents[this.toolName] || null;
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

.tool-not-found {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4em 2em;
  color: var(--textSecondary);
  text-align: center;
}

.tool-not-found i {
  font-size: 4em;
  opacity: 0.5;
  margin-bottom: 0.5em;
}

.tool-not-found h2 {
  margin: 0.5em 0;
  color: var(--textPrimary);
}
</style>
