<template>
  <div class="tools-container">
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
          <h3 style="margin:0; padding:0;">{{ $t(tool.name) }}</h3>
          <p>{{ $t(tool.description) }}</p>
        </div>
      </router-link>
    </div>
  </div>
</template>

<script>
export default {
  name: "Tools",
  data() {
    return {
      tools: [
        {
          name: "tools.sizeAnalyzer.name",
          description: "tools.sizeAnalyzer.description",
          icon: "analytics",
          path: "/tools/sizeViewer",
        },
        // Add more tools here in the future
        // {
        //   name: "tools.duplicateFinder.name",
        //   description: "tools.duplicateFinder.description",
        //   icon: "content_copy",
        //   path: "/tools/duplicateFinder",
        // },
      ],
    };
  },
};
</script>

<style scoped>
.tools-container {
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
