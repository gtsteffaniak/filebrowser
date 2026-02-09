<template>
  <div class="card-content sidebar-links-content">
    <p v-if="!showAddForm">{{ contextDescription }}</p>

    <!-- Existing Links List - only show when not in edit/add mode -->
    <div v-if="!showAddForm && !isSelectingPath" class="links-list">
      <h3>{{ $t('sidebar.currentLinks') }}</h3>
      <div v-if="links.length === 0" class="empty-state">
        <p>{{ $t('sidebar.noLinksYet') }}</p>
      </div>
      <div class="links-container">
        <div v-for="(link, index) in links" :key="index" :ref="el => linkItemRefs[index] = el"
          class="link-item input no-select" :class="{ 'dragging': draggingIndex === index }"
          @dragover.prevent="handleDragOver($event, index)" @drop="handleDrop($event, index)">
          <div draggable="true" @dragstart="handleDragStart($event, index)" @dragend="handleDragEnd"
            class="link-drag-handle">
            <i class="material-icons">drag_indicator</i>
          </div>
          <div class="link-icon">
            <i :class="getIconClass(link.icon)">{{ link.icon }}</i>
          </div>
          <div class="link-details">
            <span class="link-name">{{ getLinkDisplayName(link) }}</span>
            <span class="link-category">{{ getCategoryLabel(link.category) }}</span>
          </div>
          <button class="action input" @click="editLink(index)" :aria-label="$t('general.edit')">
            <i class="material-icons">edit</i>
          </button>
          <button class="action input" @click="removeLink(index)" :aria-label="$t('general.delete')">
            <i class="material-icons">delete</i>
          </button>
        </div>
      </div>
    </div>

    <!-- Add New Link Section -->
    <div v-if="!showAddForm" class="add-link-section">
      <button @click="showAddForm = true" class="button button--flat button--blue add-link-button">
        <i class="material-icons">add</i>
        {{ $t('sidebar.addNewLink') }}
      </button>
    </div>

    <!-- Add/Edit Link Form - replaces the list when active -->
    <div v-else class="add-link-form">
      <!-- Path Browser for Source/Share Links - shown when selecting path -->
      <div v-if="isSelectingPath">
        <file-list ref="fileList" :browse-source="newLink.category === 'source' ? newLink.sourceName : null"
          :browse-share="newLink.category === 'share' ? getShareHash(newLink.target) : null"
          @update:selected="updateSelectedPath"></file-list>
      </div>

      <!-- Form fields - hidden when selecting path -->
      <div v-else>
        <h3>{{ editingIndex !== null ? $t('sidebar.editLink') : $t('sidebar.addNewLink') }}</h3>

        <!-- Link Type Selection -->
        <p>{{ $t('sidebar.linkType') }}</p>
        <select aria-label="Link Type" v-model="newLink.category" @change="handleCategoryChange" class="input">
          <option value="">{{ $t('sidebar.selectLinkType') }}</option>
          <option v-if="context === 'user'" value="source">{{ $t('general.source') }}</option>
          <option value="share">{{ $t('general.share') }}</option>
          <option v-if="context === 'user'" value="tool">{{ $t('general.tool') }}</option>
          <option value="custom">{{ $t('sidebar.customLink') }}</option>
          <option v-if="context === 'share'" value="shareInfo">{{ $t('share.shareInfo') }}</option>
          <option v-if="context === 'share'" value="download">{{ $t('general.download') }}</option>
        </select>

        <!-- Source Selection -->
        <div v-if="newLink.category === 'source'" class="form-group">
          <p>{{ $t('sidebar.selectSource') }}</p>
          <select v-model="newLink.sourceName" @change="handleSourceChange" class="input">
            <option value="">{{ $t('sidebar.chooseSource') }}</option>
            <option v-for="(info, name) in availableSources" :key="name" :value="name">
              {{ name }}
            </option>
          </select>

          <!-- Custom Name for Source -->
          <div class="form-group" v-if="newLink.sourceName">
            <p>{{ $t('sidebar.linkName') }}</p>
            <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
              :placeholder="$t('sidebar.linkNamePlaceholder')" />
          </div>

          <!-- Path Selection for Source - clickable path display -->
          <div v-if="newLink.sourceName">
            <div class="searchContext clickable button" @click="openPathBrowser('source')" aria-label="source-path">
              {{ $t('general.path', { suffix: ':' }) }} {{ newLink.sourcePath || '/' }}
            </div>
          </div>
        </div>

        <!-- Share Selection -->
        <div v-if="newLink.category === 'share'" class="form-group">
          <p>{{ $t('sidebar.selectShare') }}</p>
          <select v-model="newLink.target" @change="handleShareChange" class="input">
            <option value="">{{ $t('sidebar.chooseShare') }}</option>
            <option v-for="share in availableShares" :key="share.hash" :value="`/public/share/${share.hash}`">
              {{ share.hash }} {{ $t('general.of') }} {{ share.path }}
            </option>
          </select>

          <!-- Custom Name for Share -->
          <div class="form-group" v-if="newLink.target">
            <p>{{ $t('sidebar.linkName') }}</p>
            <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
              :placeholder="$t('sidebar.linkNamePlaceholder')" />
          </div>

          <!-- Path Selection for Share (subpath within the share) - clickable path display -->
          <div v-if="newLink.target">
            <div class="searchContext clickable button" @click="openPathBrowser('share')" aria-label="share-path">
              {{ $t('general.path', { suffix: ':' }) }} {{ getShareSubpath(newLink.target) }}
            </div>
          </div>
        </div>

        <!-- Tool Selection - only available for user context, not shares -->
        <div v-if="newLink.category === 'tool' && context === 'user'" class="form-group">
          <p>{{ $t('sidebar.selectTool') }}</p>
          <select v-model="newLink.target" @change="handleToolChange" class="input">
            <option value="">{{ $t('sidebar.chooseTool') }}</option>
            <option v-for="tool in availableTools" :key="tool.path" :value="tool.path">
              {{ $t(tool.name) }}
            </option>
          </select>

          <!-- Custom Name for Tool -->
          <div class="form-group" v-if="newLink.target">
            <p>{{ $t('sidebar.linkName') }}</p>
            <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
              :placeholder="$t('sidebar.linkNamePlaceholder')" />
          </div>
        </div>

        <!-- Share Info Link - special type for shares -->
        <div v-if="newLink.category === 'shareInfo'" class="form-group">
          <p>{{ $t('share.shareInfoDescription') }}</p>
        </div>

        <!-- Download Link - special type for shares -->
        <div v-if="newLink.category === 'download'" class="form-group">
          <p>{{ $t('share.downloadDescription') }}</p>
        </div>

        <!-- Custom Link Input -->
        <div v-if="newLink.category === 'custom'" class="form-group">
          <p>{{ $t('sidebar.linkName') }}</p>
          <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
            :placeholder="$t('sidebar.linkNamePlaceholder')" />

          <p>{{ $t('sidebar.linkUrl') }}</p>
          <input aria-label="Link Target" v-model="newLink.target" type="text" class="input"
            :placeholder="$t('sidebar.linkUrlPlaceholder')" />
        </div>

        <!-- Icon Selection - Available for ALL link types -->
        <div v-if="newLink.category" class="form-group">
          <p>{{ $t('sidebar.linkIcon') }}</p>
          <div class="icon-input-group">
            <input v-model="newLink.icon" type="text" class="input icon-input"
              :placeholder="$t('sidebar.linkIconPlaceholder')" />
            <div class="icon-preview clickable" @click="openIconPicker" :title="$t('sidebar.browseIcons')">
              <i v-if="newLink.icon" :class="getIconClass(newLink.icon)">{{ newLink.icon }}</i>
              <i v-else class="material-icons icon-preview-placeholder">interests</i>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div class="card-actions">
    <!-- When selecting a path -->
    <template v-if="isSelectingPath">
      <button @click="cancelPathSelection" class="button button--flat button--grey" :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button @click="confirmPathSelection" class="button button--flat button--blue" :aria-label="$t('general.ok')"
        :title="$t('general.ok')">
        {{ $t("general.ok") }}
      </button>
    </template>

    <!-- When in add/edit form mode -->
    <template v-else-if="showAddForm">
      <button @click="cancelAddLink" class="button button--flat button--grey" :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button aria-label="Add Link" @click="addLink" class="button button--flat button--blue"
        :disabled="!isNewLinkValid" :title="editingIndex !== null ? $t('general.save') : $t('general.add')">
        {{ editingIndex !== null ? $t('general.save') : $t('general.add') }}
      </button>
    </template>

    <!-- When viewing the list -->
    <template v-else>
      <button aria-label="Save Links" class="button button--flat button--blue" @click="saveLinks"
        :title="$t('general.save')">
        {{ $t("general.save") }}
      </button>
    </template>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import { notify } from "@/notify";
import { usersApi, shareApi } from "@/api";
import { tools } from "@/utils/constants";
import { getIconClass } from "@/utils/material-icons";
import FileList from "../files/FileList.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "SidebarLinks",
  components: {
    FileList,
  },
  props: {
    context: {
      type: String,
      default: "user", // 'user' or 'share'
    },
    shareData: {
      type: Object,
      default: null, // Share object when context is 'share'
    },
  },
  data() {
    return {
      links: [],
      showAddForm: false,
      newLink: {
        name: "",
        category: "",
        target: "",
        icon: "",
        sourceName: "",
      },
      draggingIndex: null,
      dragOverIndex: null,
      linkItemRefs: {},
      originalLinks: null, // Store original order in case drag is cancelled
      availableTools: [
        { name: 'tools.title', path: '/tools', icon: 'build' }, // Main tools page
        ...tools() // Individual tools
      ],
      availableShares: [],
      editingIndex: null,
      isSelectingPath: false,
      tempSelectedPath: "",
      tempSelectedSource: "",
    };
  },
  computed: {
    availableSources() {
      return state.sources?.info || {};
    },
    currentTitle() {
      // Always show the context title, path selection is shown inline
      return this.contextTitle;
    },
    contextTitle() {
      return this.context === 'share'
        ? this.$t('sidebar.customizeShareLinks')
        : this.$t('sidebar.customizeLinks');
    },
    contextDescription() {
      return this.context === 'share'
        ? this.$t('sidebar.customizeShareLinksDescription')
        : this.$t('sidebar.customizeLinksDescription');
    },
    isNewLinkValid() {
      if (!this.newLink.category) return false;

      // Special link types for shares don't need additional validation
      if (this.newLink.category === "shareInfo" || this.newLink.category === "download") {
        return true;
      }

      if (this.newLink.category === "custom") {
        return this.newLink.name && this.newLink.target;
      }

      if (this.newLink.category === "source") {
        return this.newLink.sourceName && this.newLink.name;
      }

      if (this.newLink.category === "share") {
        return this.newLink.target && this.newLink.name;
      }

      return this.newLink.target && this.newLink.name;
    },
  },
  async mounted() {
    // Initialize with existing sidebar links based on context
    if (this.context === 'share' && this.shareData?.sidebarLinks) {
      this.links = [...this.shareData.sidebarLinks];
    } else if (this.context === 'user' && state.user?.sidebarLinks && state.user.sidebarLinks.length > 0) {
      this.links = [...state.user.sidebarLinks];
    } else if (this.context === 'user') {
      // Generate default links from sources for user context
      this.links = this.getDefaultLinks();
    }

    // Load available shares for share link type
    if (this.context === 'user') {
      await this.loadAvailableShares();
    }
  },
  methods: {
    getIconClass,
    getShareHash(target) {
      // Extract hash from /public/share/<hash> or /public/share/<hash>/path
      if (!target) return '';
      const parts = target.split('/');
      // parts: ['', 'public', 'share', '<hash>', ...subpath]
      if (parts.length >= 4 && parts[1] === 'public' && parts[2] === 'share') {
        return parts[3];
      }
      return '';
    },
    getShareSubpath(target) {
      // Extract subpath from /public/share/<hash>/subpath
      if (!target) return '/';
      const parts = target.split('/');
      // parts: ['', 'public', 'share', '<hash>', ...subpath]
      if (parts.length >= 4 && parts[1] === 'public' && parts[2] === 'share') {
        return parts.length > 4 ? '/' + parts.slice(4).join('/') : '/';
      }
      return '/';
    },
    closePrompt() {
      // Close only this prompt (SidebarLinks), returning to the previous one (Share)
      mutations.closeTopHover();
    },
    openIconPicker() {
      mutations.showHover({
        name: "IconPicker",
        props: {
          onSelect: this.handleIconSelect,
        },
      });
    },
    handleIconSelect(iconName) {
      this.newLink.icon = iconName;
    },
    async loadAvailableShares() {
      try {
        this.availableShares = await shareApi.list();
      } catch (error) {
        console.error("Failed to load shares:", error);
        this.availableShares = [];
      }
    },
    getDefaultLinks() {
      // Generate default links from sources
      const defaultLinks = [];

      if (this.availableSources) {
        Object.keys(this.availableSources).forEach(sourceName => {
          defaultLinks.push({
            name: sourceName,
            category: 'source',
            target: '/', // Relative path to source root
            icon: '', // No icon by default - will show animated status indicator
            sourceName: sourceName,
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
        share: this.$t('general.share'),
        shareInfo: this.$t('share.shareInfo'),
        download: this.$t('general.download'),
      };
      return labels[category] || category;
    },
    getLinkDisplayName(link) {
      // Check if the name looks like a translation key that needs translating
      if (link.category === 'shareInfo' && link.name === 'share.shareInfo') {
        return this.$t('share.shareInfo');
      }
      if (link.category === 'download' && link.name === 'general.download') {
        return this.$t('general.download');
      }
      // Check if it's a general translation key pattern
      if (typeof link.name === 'string' && link.name.includes('.') && link.name.split('.').length === 2) {
        // Try to translate, if it fails, return original
        try {
          const translated = this.$t(link.name);
          // If translation returns the same key, it means it doesn't exist, return original
          return translated !== link.name ? translated : link.name;
        } catch (e) {
          return link.name;
        }
      }
      return link.name;
    },
    handleCategoryChange() {
      // Reset newLink fields when category changes
      this.newLink.name = "";
      this.newLink.target = "";
      this.newLink.icon = "";
      this.newLink.sourceName = "";
      this.newLink.sourcePath = "";

      // Set default name and icon for special share link types
      if (this.newLink.category === "shareInfo") {
        this.newLink.name = this.$t("share.shareInfo");
        this.newLink.icon = "qr_code";
      } else if (this.newLink.category === "download") {
        this.newLink.name = this.$t("general.download");
        this.newLink.icon = "download";
      }
    },
    handleSourceChange() {
      if (this.newLink.sourceName) {
        // Only set default name if user hasn't entered one yet
        if (!this.newLink.name) {
          this.newLink.name = this.newLink.sourceName;
        }
        // No icon by default - will show animated status indicator
        if (!this.newLink.icon) {
          this.newLink.icon = "";
        }
        this.newLink.sourcePath = "/";
      }
    },
    handleShareChange() {
      if (this.newLink.target) {
        const hash = this.getShareHash(this.newLink.target);
        const share = this.availableShares.find(s => s.hash === hash);
        if (share) {
          // Only set default name if user hasn't entered one yet
          if (!this.newLink.name) {
            this.newLink.name = `Share: ${share.hash}`;
          }
          // Suggest default icon if not set
          if (!this.newLink.icon) {
            this.newLink.icon = "share";
          }
        }
      }
    },
    handleToolChange() {
      const tool = this.availableTools.find(t => t.path === this.newLink.target);
      if (tool) {
        // Only set default name and icon if user hasn't entered them yet
        if (!this.newLink.name) {
          this.newLink.name = this.$t(tool.name);
        }
        if (!this.newLink.icon) {
          this.newLink.icon = tool.icon;
        }
      }
    },
    openPathBrowser(type) {
      // Show file list for path browsing
      this.isSelectingPath = true;
      if (type === 'source') {
        this.tempSelectedPath = this.newLink.sourcePath || '/';
        this.tempSelectedSource = this.newLink.sourceName;
      } else if (type === 'share') {
        this.tempSelectedPath = this.getShareSubpath(this.newLink.target);
        this.tempSelectedSource = '';
      }
    },
    updateSelectedPath(pathOrData) {
      // Handle both old format (string) and new format (object with path and source)
      if (typeof pathOrData === 'string') {
        this.tempSelectedPath = pathOrData;
      } else if (pathOrData && pathOrData.path) {
        this.tempSelectedPath = pathOrData.path;
        this.tempSelectedSource = pathOrData.source;
      }
    },
    confirmPathSelection() {
      // Apply the selected path to the link based on category
      if (this.newLink.category === 'source') {
        this.newLink.sourcePath = this.tempSelectedPath;
      } else if (this.newLink.category === 'share') {
        // Update target with new subpath
        const hash = this.getShareHash(this.newLink.target);
        const subpath = this.tempSelectedPath === '/' ? '' : this.tempSelectedPath;
        this.newLink.target = `/public/share/${hash}${subpath}`;
      }
      this.isSelectingPath = false;
    },
    cancelPathSelection() {
      // Cancel path selection and return to form
      this.isSelectingPath = false;
      this.tempSelectedPath = "";
      this.tempSelectedSource = "";
    },
    editLink(index) {
      const link = this.links[index];
      this.editingIndex = index;
      this.showAddForm = true;

      // Populate form with existing link data
      this.newLink = {
        name: link.name,
        category: link.category,
        target: (link.category === 'source') ? "" : (link.target || ""),
        icon: link.icon || "",
        sourceName: link.sourceName || "",
        sourcePath: (link.category === 'source') ? (link.target || "/") : "/",
      };
    },
    addLink() {
      if (!this.isNewLinkValid) return;

      // Build the link object based on category
      // Always include: name, category, target, icon, and conditionally sourceName/shareHash
      const linkData = {
        name: this.newLink.name,
        category: this.newLink.category,
        icon: this.newLink.icon,
        target: "",
      };

      // Process target and additional fields based on category
      if (this.newLink.category === "shareInfo") {
        // ShareInfo is a special action link
        linkData.target = "#";
      } else if (this.newLink.category === "download") {
        // Download is a special action link
        linkData.target = "#";
      } else if (this.newLink.category === "custom") {
        linkData.target = this.processCustomUrl(this.newLink.target);
      } else if (this.newLink.category === "source") {
        // For sources: target is relative path, sourceName identifies the source
        linkData.target = this.newLink.sourcePath || '/';
        linkData.sourceName = this.newLink.sourceName;
      } else if (this.newLink.category === "share") {
        // For shares: target is already the full path /public/share/<hash>/<subpath>
        linkData.target = this.newLink.target;
      } else if (this.newLink.category === "tool") {
        linkData.target = this.newLink.target;
      }

      if (this.editingIndex !== null) {
        // Update existing link
        this.links[this.editingIndex] = linkData;
      } else {
        // Add new link
        this.links.push(linkData);
      }

      // Close the form and return to list view
      this.cancelAddLink();
    },
    processCustomUrl(url) {      
      // Check if it's an external URL (case insensitive)
      const lowerUrl = url.toLowerCase();
      if (lowerUrl.startsWith('http://') || lowerUrl.startsWith('https://')) {
        // Leave external URLs as-is
        return url;
      }
      // For internal paths, normalize by adding a starting slash if missing
      if (!url.startsWith('/')) {
        return '/' + url; // assume relative path
      }
      return url;
    },
    removeLink(index) {
      this.links.splice(index, 1);
    },
    cancelAddLink() {
      this.showAddForm = false;
      this.editingIndex = null;
      this.newLink = {
        name: "",
        category: "",
        target: "",
        icon: "",
        sourceName: "",
        sourcePath: "",
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
        if (this.context === 'share') {
          // Save to share
          const payload = {
            hash: this.shareData.hash,
            sidebarLinks: this.links,
          };

          await shareApi.create(payload);

          // Notify Share component of the updated links via eventBus
          eventBus.emit('shareSidebarLinksUpdated', {
            hash: this.shareData.hash,
            sidebarLinks: this.links,
          });

        } else {
          // Save to user
          const updatedUser = {
            id: state.user.id,
            username: state.user.username,
            sidebarLinks: this.links,
          };

          await usersApi.update(updatedUser, ['sidebarLinks']);

          // Update the local state
          state.user.sidebarLinks = [...this.links];

          notify.showSuccessToast(this.$t("sidebar.linksUpdatedSuccess"));
        }

        // Close only this prompt, returning to the previous one (if any)
        mutations.closeTopHover();
      } catch (error) {
        notify.showError(this.$t("sidebar.linksUpdateFailed"));
      }
    },
  },
};
</script>

<style scoped>
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
  margin-top: 0;
}

.form-group p {
  margin: 0.5em
}

.form-group p:first-of-type,
.add-link-form>p:first-of-type {
  margin-top: 0;
}

/* Icon preview styles */
.icon-input-group {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.icon-input {
  flex: 1;
}

.icon-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3em;
  height: 3em;
  border: 1px solid var(--borderColor);
  border-radius: 1em;
  background: var(--surfaceSecondary);
  color: var(--primaryColor);
}

.icon-preview .material-icons,
.icon-preview .material-symbols-outlined {
  font-size: 2em;
}

.icon-preview-placeholder {
  color: var(--textSecondary);
  opacity: 0.6;
}
</style>
