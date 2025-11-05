<template>
    <div class="card-title">
        <h2>{{ $t('sidebar.customizeLinks') }}</h2>
    </div>

    <div class="card-content sidebar-links-content">
        <p>{{ $t('sidebar.customizeLinksDescription') }}</p>

        <!-- Existing Links List -->
        <div class="links-list">
            <h3>{{ $t('sidebar.currentLinks') }}</h3>
            <div v-if="links.length === 0" class="empty-state">
                <p>{{ $t('sidebar.noLinksYet') }}</p>
            </div>
            <div class="links-container">
                <div v-for="(link, index) in links" :key="index"
                    :ref="el => linkItemRefs[index] = el"
                    class="link-item input no-select"
                    :class="{ 'dragging': draggingIndex === index }"
                    @dragover.prevent="handleDragOver($event, index)"
                    @drop="handleDrop($event, index)">
                    <div
                        draggable="true"
                        @dragstart="handleDragStart($event, index)"
                        @dragend="handleDragEnd"
                        class="link-drag-handle">
                        <i class="material-icons">drag_indicator</i>
                    </div>
                    <div class="link-icon">
                        <i class="material-icons">{{ link.icon }}</i>
                    </div>
                    <div class="link-details">
                        <span class="link-name">{{ link.name }}</span>
                        <span class="link-category">{{ getCategoryLabel(link.category) }}</span>
                    </div>
                    <button class="action input" @click="removeLink(index)"
                        :aria-label="$t('general.delete')">
                        <i class="material-icons">delete</i>
                    </button>
                </div>
            </div>
        </div>

        <!-- Add New Link Section -->
        <div class="add-link-section">
            <button v-if="!showAddForm" @click="showAddForm = true"
                class="button button--flat button--blue add-link-button">
                <i class="material-icons">add</i>
                {{ $t('sidebar.addNewLink') }}
            </button>

            <div v-else class="add-link-form">
                <h3>{{ $t('sidebar.addNewLink') }}</h3>

                <!-- Link Type Selection -->
                <p>{{ $t('sidebar.linkType') }}</p>
                <select v-model="newLink.category" @change="handleCategoryChange" class="input">
                    <option value="">{{ $t('sidebar.selectLinkType') }}</option>
                    <option value="source">{{ $t('general.source') }}</option>
                    <option value="tool">{{ $t('general.tool') }}</option>
                    <option value="custom">{{ $t('sidebar.customLink') }}</option>
                </select>

                <!-- Source Selection -->
                <div v-if="newLink.category === 'source'" class="form-group">
                    <p>{{ $t('sidebar.selectSource') }}</p>
                    <select v-model="newLink.target" @change="handleSourceChange" class="input">
                        <option value="">{{ $t('sidebar.chooseSource') }}</option>
                        <option v-for="(info, name) in availableSources" :key="name" :value="name">
                            {{ name }}
                        </option>
                    </select>
                </div>

                <!-- Tool Selection -->
                <div v-if="newLink.category === 'tool'" class="form-group">
                    <p>{{ $t('sidebar.selectTool') }}</p>
                    <select v-model="newLink.target" @change="handleToolChange" class="input">
                        <option value="">{{ $t('sidebar.chooseTool') }}</option>
                        <option v-for="tool in availableTools" :key="tool.path" :value="tool.path">
                            {{ $t(tool.name) }}
                        </option>
                    </select>
                </div>

                <!-- Custom Link Input -->
                <div v-if="newLink.category === 'custom'" class="form-group">
                    <p>{{ $t('sidebar.linkName') }}</p>
                    <input v-model="newLink.name" type="text" class="input"
                        :placeholder="$t('sidebar.linkNamePlaceholder')" />

                    <p>{{ $t('sidebar.linkUrl') }}</p>
                    <input v-model="newLink.target" type="text" class="input"
                        :placeholder="$t('sidebar.linkUrlPlaceholder')" />

                    <p>{{ $t('sidebar.linkIcon') }}</p>
                    <input v-model="newLink.icon" type="text" class="input"
                        :placeholder="$t('sidebar.linkIconPlaceholder')" />
                </div>

                <!-- Add/Cancel Buttons for Form -->
                <div class="form-actions">
                    <button @click="cancelAddLink" class="button button--flat button--grey">
                        {{ $t('general.cancel') }}
                    </button>
                    <button @click="addLink" class="button button--flat button--blue" :disabled="!isNewLinkValid">
                        {{ $t('general.add') }}
                    </button>
                </div>
            </div>
        </div>
    </div>

    <div class="card-action">
        <button @click="closeHovers" class="button button--flat button--grey" :aria-label="$t('general.cancel')"
            :title="$t('general.cancel')">
            {{ $t("general.cancel") }}
        </button>
        <button class="button button--flat button--blue" @click="saveLinks" :title="$t('general.save')">
            {{ $t("general.save") }}
        </button>
    </div>
</template>

<script>
import { state, mutations } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";

export default {
  name: "SidebarLinks",
  data() {
    return {
      links: [],
      showAddForm: false,
      newLink: {
        name: "",
        category: "",
        target: "",
        icon: "",
      },
      draggingIndex: null,
      dragOverIndex: null,
      linkItemRefs: {},
      originalLinks: null, // Store original order in case drag is cancelled
      availableTools: [
        {
          name: "tools.sizeAnalyzer.name",
          path: "/tools/sizeViewer",
          icon: "analytics",
        },
      ],
    };
  },
  computed: {
    availableSources() {
      return state.sources?.info || {};
    },
    isNewLinkValid() {
      if (!this.newLink.category) return false;

      if (this.newLink.category === "custom") {
        return this.newLink.name && this.newLink.target && this.newLink.icon;
      }

      return this.newLink.target && this.newLink.name;
    },
  },
  mounted() {
    // Initialize with existing sidebar links or generate defaults from sources
    if (state.user?.sidebarLinks && state.user.sidebarLinks.length > 0) {
      this.links = [...state.user.sidebarLinks];
    } else {
      // Generate default links from sources
      this.links = this.getDefaultLinks();
    }
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    getDefaultLinks() {
      // Generate default links from sources
      const defaultLinks = [];

      if (this.availableSources) {
        Object.keys(this.availableSources).forEach(sourceName => {
          const info = this.availableSources[sourceName];
          defaultLinks.push({
            name: sourceName,
            category: 'source',
            target: `/files/${info.pathPrefix}`,
            icon: 'folder',
          });
        });
      }

      return defaultLinks;
    },
    getCategoryLabel(category) {
      const labels = {
        source: this.$t('general.source'),
        tool: this.$t('general.tool'),
        custom: this.$t('sidebar.customLink'),
      };
      return labels[category] || category;
    },
    handleCategoryChange() {
      // Reset newLink fields when category changes
      this.newLink.name = "";
      this.newLink.target = "";
      this.newLink.icon = "";
    },
    handleSourceChange() {
      if (this.newLink.target) {
        const sourceName = this.newLink.target;
        this.newLink.name = sourceName;
        this.newLink.icon = "folder";
      }
    },
    handleToolChange() {
      const tool = this.availableTools.find(t => t.path === this.newLink.target);
      if (tool) {
        this.newLink.name = this.$t(tool.name);
        this.newLink.icon = tool.icon;
      }
    },
    addLink() {
      if (!this.isNewLinkValid) return;

      // Process target based on category
      let target = this.newLink.target;
      if (this.newLink.category === "custom") {
        target = this.processCustomUrl(target);
      } else if (this.newLink.category === "source") {
        // For sources, build the path from the source info
        const sourceName = this.newLink.target;
        const sourceInfo = this.availableSources[sourceName];
        // Build path: /files/ or /files/encodedSourceName
        target = sourceInfo.pathPrefix ? `/files/${sourceInfo.pathPrefix}` : '/files/';
      }

      this.links.push({
        name: this.newLink.name,
        category: this.newLink.category,
        target: target,
        icon: this.newLink.icon,
      });

      this.cancelAddLink();
    },
    processCustomUrl(url) {
      try {
        // If it's a full URL, try to extract the path
        if (url.startsWith('http://') || url.startsWith('https://')) {
          const urlObj = new URL(url);
          const pathname = urlObj.pathname;

          // Remove subpath if present
          const subpath = window.location.pathname.split('/')[1];
          if (subpath && pathname.startsWith('/' + subpath)) {
            return pathname.substring(subpath.length + 1);
          }

          return pathname;
        }

        // If it's already a path, return as is
        return url;
      } catch (e) {
        // If URL parsing fails, return as is
        return url;
      }
    },
    removeLink(index) {
      this.links.splice(index, 1);
    },
    cancelAddLink() {
      this.showAddForm = false;
      this.newLink = {
        name: "",
        category: "",
        target: "",
        icon: "",
      };
    },
    handleDragStart(event, index) {
      this.draggingIndex = index;
      this.dragOverIndex = null;

      // Store original order in case drag is cancelled
      this.originalLinks = [...this.links];

      event.dataTransfer.effectAllowed = "move";
      event.dataTransfer.setData("text/html", event.target);

      // Set the entire link item as the drag image
      const linkItem = this.linkItemRefs[index];
      if (linkItem) {
        // Create a clone for the drag image to avoid affecting the original
        const clone = linkItem.cloneNode(true);
        clone.style.position = 'absolute';
        clone.style.top = '-9999px';
        clone.style.left = '-9999px';

        // Set the clone width to match the original element's width
        const originalWidth = linkItem.offsetWidth;
        clone.style.width = `${originalWidth}px`;

        document.body.appendChild(clone);

        // Set it as the drag image
        event.dataTransfer.setDragImage(clone, event.offsetX, event.offsetY);

        // Clean up the clone after a brief delay
        setTimeout(() => {
          document.body.removeChild(clone);
        }, 0);
      }
    },
    handleDragOver(event, index) {
      if (this.draggingIndex === null || this.draggingIndex === index) return;

      // Only reorder if we're hovering over a different item
      if (this.dragOverIndex !== index) {
        this.dragOverIndex = index;

        // Live reorder: move the dragged item to the new position
        const newLinks = [...this.links];
        const draggedLink = newLinks[this.draggingIndex];

        // Remove from current position
        newLinks.splice(this.draggingIndex, 1);

        // Insert at hover position
        newLinks.splice(index, 0, draggedLink);

        // Update the array and dragging index
        this.links = newLinks;
        this.draggingIndex = index; // Update to new position
      }
    },
    handleDrop(event) {
      event.preventDefault();

      // The array is already in the correct order from handleDragOver
      // Just clean up the drag state
      this.draggingIndex = null;
      this.dragOverIndex = null;
      this.originalLinks = null; // Clear the backup
    },
    handleDragEnd() {
      // If drag was cancelled (no drop event), restore original order
      if (this.originalLinks !== null) {
        this.links = this.originalLinks;
        this.originalLinks = null;
      }

      this.draggingIndex = null;
      this.dragOverIndex = null;
    },
    async saveLinks() {
      try {
        const updatedUser = {
          id: state.user.id,
          username: state.user.username,
          sidebarLinks: this.links,
        };

        await usersApi.update(updatedUser, ['sidebarLinks']);

        // Update the local state
        state.user.sidebarLinks = [...this.links];

        notify.showSuccess(this.$t("sidebar.linksUpdatedSuccess"));
        mutations.closeHovers();
      } catch (error) {
        notify.showError(this.$t("sidebar.linksUpdateFailed"));
      }
    },
  },
};
</script>

<style scoped>
/* Component-specific layout */
.sidebar-links-content {
    max-height: 60vh;
    overflow-y: auto;
}

.links-list {
    margin-bottom: 1.5em;
}

.links-list h3,
.add-link-form h3 {
    margin-bottom: 0.5em;
    font-size: 1em;
    font-weight: 600;
}

.empty-state {
    padding: 2em 1em;
    text-align: center;
    color: var(--textSecondary);
    font-style: italic;
}

.add-link-form h3 {
    margin-top: 0;
    margin-bottom: 0.75em;
    font-size: 0.95em;
}

.links-container {
    display: flex;
    flex-direction: column;
    gap: 0.5em;
}

/* Link item styles */
.link-item {
    display: flex;
    align-items: center;
    gap: 0.5em;
    background: var(--surfaceSecondary);
    transition: all 0.2s ease;
}

.link-item.dragging {
    opacity: 0.5;
    border-color: var(--primaryColor);
    background: var(--surfaceTertiary);
}

.link-drag-handle {
    color: var(--textSecondary);
    cursor: grab;
}

.link-drag-handle:active {
    cursor: grabbing;
}

.link-icon {
    color: var(--primaryColor);
}

.link-details {
    display: flex;
    flex-direction: column;
    gap: 0.25em;
    flex-grow: 1;
    width: 100%;
}

.link-name {
    font-weight: 500;
}

.link-category {
    font-size: 0.85em;
    color: var(--textSecondary);
}

/* Form sections */
.add-link-section {
    margin-top: 1.5em;
    padding-top: 1em;
    border-top: 1px solid var(--borderColor);
}

.add-link-button {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5em;
}

.add-link-form {
    padding: 0;
    margin-top: 1em;
}

.form-group {
    margin-bottom: 1em;
    margin-top: 1em;
}

.form-group p {
    margin-bottom: 0.5em;
    margin-top: 0.75em;
}

.form-group p:first-of-type,
.add-link-form>p:first-of-type {
    margin-top: 0;
}

.form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5em;
    margin-top: 1em;
}
</style>
