<template>
  <div class="tools-container">
    <div class="tools-header">
      <h1>Tools</h1>
      <p class="description">Utilities and analysis tools for your files</p>
    </div>

    <div class="tools-grid">
      <router-link
        v-for="tool in tools"
        :key="tool.path"
        :to="tool.path"
        class="tool-card"
      >
        <div class="tool-icon">
          <i class="material-icons">{{ tool.icon }}</i>
        </div>
        <div class="tool-content">
          <h3>{{ tool.name }}</h3>
          <p>{{ tool.description }}</p>
        </div>
        <div class="tool-arrow">
          <i class="material-icons">arrow_forward</i>
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
          name: "File Size Analyzer",
          description: "Visualize large files and directories with an interactive treemap",
          icon: "analytics",
          path: "/tools/sizeViewer",
        },
        // Add more tools here in the future
        // {
        //   name: "Duplicate Finder",
        //   description: "Find and manage duplicate files",
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
  padding: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.tools-header {
  margin-bottom: 3rem;
}

.tools-header h1 {
  font-size: 2.5rem;
  margin: 0 0 0.5rem 0;
  color: var(--textPrimary);
}

.tools-header .description {
  font-size: 1.1rem;
  color: var(--textSecondary);
  margin: 0;
}

.tools-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 1.5rem;
}

.tool-card {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  padding: 1.5rem;
  background: var(--surfaceSecondary);
  border-radius: 8px;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s ease;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  cursor: pointer;
}

.tool-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  background: var(--surfaceTertiary);
}

.tool-icon {
  flex-shrink: 0;
  width: 60px;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--blue) 0%, var(--blueHover) 100%);
  border-radius: 12px;
  color: white;
}

.tool-icon .material-icons {
  font-size: 2rem;
}

.tool-content {
  flex: 1;
}

.tool-content h3 {
  margin: 0 0 0.5rem 0;
  font-size: 1.25rem;
  color: var(--textPrimary);
}

.tool-content p {
  margin: 0;
  font-size: 0.95rem;
  color: var(--textSecondary);
  line-height: 1.5;
}

.tool-arrow {
  flex-shrink: 0;
  color: var(--textSecondary);
  transition: transform 0.2s ease;
}

.tool-card:hover .tool-arrow {
  transform: translateX(4px);
  color: var(--blue);
}

.tool-arrow .material-icons {
  font-size: 1.5rem;
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .tools-container {
    padding: 1rem;
  }

  .tools-header h1 {
    font-size: 2rem;
  }

  .tools-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }

  .tool-card {
    padding: 1.25rem;
  }

  .tool-icon {
    width: 50px;
    height: 50px;
  }

  .tool-icon .material-icons {
    font-size: 1.5rem;
  }

  .tool-content h3 {
    font-size: 1.1rem;
  }

  .tool-content p {
    font-size: 0.9rem;
  }
}
</style>

