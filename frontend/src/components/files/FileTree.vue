<template>
  <div ref="FileTree" class="file-tree" :class="{ 'file-tree-root': isRootInstance }">
    <!-- Loading root -->
    <div v-if="isRootInstance && loading" class="tree-loading">
      <LoadingSpinner size="small" mode="placeholder" />
    </div>
    <!-- Error state -->
    <div v-if="isRootInstance && error" class="tree-error">
      <i class="material-icons">error</i>
      <span>{{ $t("prompts.error") }}</span>
    </div>
    <ul v-if="effectiveNodes && effectiveNodes.length" class="tree-list">
      <li v-for="node in effectiveNodes" :key="node.path" class="tree-item">
        <div
          class="tree-node"
          :class="{
            'current-item': isCurrentItem(node),
            'has-children': node.childrenCount > 0,
            'drag-over': node.dragOver || isSelected(node),
          }"
          @click="handleNodeClick(node)"
          @contextmenu.prevent="handleContextMenu($event, node)"
          @dragover.prevent="handleDragOver($event, node)"
          @dragleave.prevent="handleDragLeave($event, node)"
          @drop.prevent="handleDrop($event, node)"
        >
          <!-- Expand/collapse arrow (only for folders) -->
          <span
            v-if="node.isDir"
            class="expand-icon"
            @click.stop="toggleExpand(node, true)"
          >
            <i class="material-icons">
              {{ node.expanded ? 'expand_more' : 'chevron_right' }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            </i>
          </span>
          <span v-else class="expand-icon-placeholder"></span>
          <Icon
            :mimetype="node.isDir ? 'directory' : node.type"
            :filename="node.name"
            :isShared="isShare"
            class="node-icon"
          />
          <span class="node-name" :title="node.name">
            {{ node.name }}
          </span>
        </div>
        <!-- Recursive children if expanded -->
        <FileTree
          v-if="node.expanded && node.children && node.children.length > 0"
          :nodes="node.children"
          :currentSource="currentSource"
          :shareHash="shareHash"
          :currentPath="currentPath"
        />
        <div v-else-if="node.expanded && node.children && node.children.length === 0" class="tree-empty-folder">
          {{ $t('files.lonely') }}
        </div>
        <div v-else-if="node.expanded && node.loading" class="tree-loading">
          <LoadingSpinner size="small" mode="placeholder" />
        </div>
        <div v-else-if="node.expanded && node.childrenError" class="tree-error small">
          {{ node.childrenError }}
        </div>
      </li>
    </ul>
    <!-- Empty state -->
    <div v-if="isRootInstance && effectiveNodes && effectiveNodes.length === 0 && !loading && !error" class="tree-empty">
      <i class="material-icons">sentiment_dissatisfied</i>
      {{ $t('files.lonely') }}
    </div>
  </div>
</template>

<script>
import { resourcesApi } from '@/api';
import Icon from '@/components/files/Icon.vue';
import LoadingSpinner from '@/components/LoadingSpinner.vue';
import { state, getters, mutations } from '@/store';
import { goToItem, joinPath } from '@/utils/url';
import { notify } from '@/notify';

export default {
  name: 'FileTree',
  components: { Icon, LoadingSpinner },
  props: {
    // Will be ignored if we are in a share
    currentSource: {
      type: String,
      default: null
    },
    // Nodes to render (for recursion, root uses its own data and leaves this null)
    // with 'root', I mean whole tree view component, the one that contains all the nodes/folders.
    nodes: {
      type: Array,
      default: null
    },
    currentPath: {
      type: String,
      default: '/'
    },
    // Root path to start from
    rootPath: {
      type: String,
      default: '/'
    },
  },
  data() {
    return {
      rootNodes: [],
      loading: false,
      error: null,
      expanding: false,
      isRefreshing: false,
      unwatchReload: null,
      expandTimeouts: new Map(), // For auto-expand when dragging a item to a folder
    };
  },
  computed: {
    isShare() {
      return !!this.shareHash;
    },
    shareHash() {
      return getters.shareHash() || null;
    },
    // Determine if this instance is the root of the tree
    isRootInstance() {
      return this.nodes === null;
    },
    effectiveNodes() {
      return this.isRootInstance ? this.rootNodes : this.nodes;
    },
    canModify() {
      return getters.permissions()?.modify;
    },
  },
  watch: {
    currentSource: {
      handler(newVal, oldVal) {
        if (this.isRootInstance && newVal !== oldVal) this.loadRoot();
      },
      immediate: true,
    },
    shareHash: {
      handler(newVal, oldVal) {
        if (this.isRootInstance && newVal !== oldVal) this.loadRoot();
      },
      immediate: true,
    },
    rootPath: {
      handler(newVal, oldVal) {
        if (this.isRootInstance && newVal !== oldVal) this.loadRoot();
      },
    },
    currentPath: {
      async handler(newPath, oldPath) {
        if (this.isRootInstance && newPath && newPath !== oldPath) {
          await this.expandToPath(newPath);
        }
      },
      immediate: true,
    },
  },
  mounted() {
    this.unwatchReload = this.$watch(
      () => state.reload,
      (newVal) => {
        if (newVal === true && this.isRootInstance && !this.isRefreshing) {
          console.log('Reload detected, refreshing tree');
          this.refresh();
        }
      },
      { immediate: false, flush: 'sync' }
    );
    window.addEventListener('dragend', this.clearAllDragStates);
  },
  beforeUnmount() {
    // Clear all expand timeouts
    this.expandTimeouts.forEach(timeout => clearTimeout(timeout));
    this.expandTimeouts.clear();
    if (this.unwatchReload) {
      this.unwatchReload();
    }
    window.removeEventListener('dragend', this.clearAllDragStates);
  },
  methods: {
    async loadRoot() {
      this.error = null;
      if ((!this.currentSource && !this.shareHash) || !this.isRootInstance) {
        this.rootNodes = [];
        return;
      }
      this.loading = true;
      try {
        const items = await this.fetchItems(this.rootPath);
        this.rootNodes = items.map(item => this.createNode(item));
      } catch (err) {
        console.error('Failed to load tree root:', err);
        this.error = err.message || 'Failed to load';
        this.rootNodes = [];
      } finally {
        this.loading = false;
      }
    },
    async fetchItems(path) {
      if (this.isShare) {
        const res = await resourcesApi.fetchFilesPublic(path, this.shareHash, state.shareInfo.password, false, false, true);
        return res.items || [];
      } else {
        const res = await resourcesApi.fetchFiles(this.currentSource, path, false, false, true);
        return res.items || [];
      }
    },

    createNode(item) {
      return {
        name: item.name,
        path: item.path,
        source: this.currentSource,
        type: item.type,
        isDir: item.type === 'directory',
        expanded: false,
        loading: false,
        children: null,
        childrenCount: 0,
        childrenError: null,
        dragOver: false, // For drag highlight
        ...item
      };
    },

    async toggleExpand(node, skipNavigate = false) {
      if (!node.isDir) return;

      if (node.expanded) {
        node.expanded = false;
        return;
      }

      // Immediately expand and show loading state
      node.expanded = true;
      
      // Load children if not loaded
      if (!node.children) {
        node.loading = true;
        node.childrenError = null;
        try {
          const items = await this.fetchItems(node.path);
          node.children = items.map(item => this.createNode(item));
          node.childrenCount = node.children.length;
        } catch (error) {
          console.error('Failed to load children for', node.path, error);
          node.children = [];
          node.childrenCount = 0;
          node.childrenError = error.message || 'Failed to load';
        } finally {
          node.loading = false;
        }
      }

      if (!skipNavigate && node.path !== this.currentPath) {
        this.navigateTo(node);
      }
    },

    handleNodeClick(node) {
      if (node.isDir) {
        if (!this.isCurrentItem(node)) {
          this.navigateTo(node);
        } else {
          this.toggleExpand(node, false);
        }
      } else {
        this.navigateTo(node);
      }
    },

    handleContextMenu(event, node) {
      event.preventDefault();
      event.stopPropagation();

      // Build an array with the single node (we don't support multi-select in tree yet, maybe later! But I'm not sure tbh)
      const items = [node];
      mutations.showHover({
        name: 'ContextMenu',
        props: {
          posX: event.clientX,
          posY: event.clientY,
          items: items, // Pass to context menu
        },
      });
    },

    navigateTo(node) {
      if (this.isShare) {
        goToItem(null, node.path, {}, false, this.shareHash);
      } else {
        goToItem(this.currentSource, node.path, {});
      }
    },

    isCurrentItem(node) {
      if (this.isShare) {
        return node.path === this.currentPath;
      } else {
        return node.source === this.currentSource && node.path === this.currentPath;
      }
    },

    async expandToPath(targetPath) {
      if (!targetPath || targetPath === '/' || this.expanding) return;
      this.expanding = true;
      try {
        if (this.isRootInstance && !this.rootNodes.length) {
          await this.loadRoot();
        }
        const pathParts = targetPath.split('/').filter(p => p);
        if (pathParts.length === 0) return;

        let currentLevel = this.isRootInstance ? this.rootNodes : this.nodes;

        for (let i = 0; i < pathParts.length; i++) {
          const part = pathParts[i];
          const node = currentLevel.find(n => n.name === part && n.isDir);
          if (!node) break;

          if (!node.expanded) {
            await this.toggleExpand(node, true);
          }
          currentLevel = node.children || [];
        }

        await this.$nextTick();
        this.scrollToCurrent();
      } catch (err) {
        console.error('Error expanding path:', err);
      } finally {
        this.expanding = false;
      }
    },

    scrollToCurrent() {
      if (!this.$refs.FileTree) return;
      const current = this.$refs.FileTree.querySelector('.tree-node.current-item');
      if (current) {
        current.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    },

    async refresh() {
      if (!this.isRootInstance || this.isRefreshing) return;
      this.isRefreshing = true;
      this.expandTimeouts.clear();
      // Save expanded paths before reload to restore them
      const expandedPaths = this.collectExpandedPaths(this.rootNodes);

      try {
        await this.loadRoot();
        // Restore expanded paths
        await this.applyExpandedPaths(this.rootNodes, expandedPaths);
        // Ensure current directory is expanded (it might already be)
        await this.expandToPath(this.currentPath);
        await this.scrollToCurrent();
      } finally {
        this.isRefreshing = false;
      }
    },
    collectExpandedPaths(nodes, set = new Set()) {
      for (const node of nodes) {
        if (node.expanded) {
          set.add(node.path);
        }
        if (node.children) {
          this.collectExpandedPaths(node.children, set);
        }
      }
      return set;
    },
    async applyExpandedPaths(nodes, expandedPaths) {
      for (const node of nodes) {
        if (expandedPaths.has(node.path)) {
          // Expand the node (load children if needed)
          await this.toggleExpand(node, true);
          // Recursively apply to children (if any)
          if (node.children) {
            await this.applyExpandedPaths(node.children, expandedPaths);
          }
        }
      }
    },
    handleDragOver(event, node) {
      if (!node.isDir || !this.canModify) return;

      const isInternal = Array.from(event.dataTransfer.types).includes(
        'application/x-filebrowser-internal-drag'
      );
      if (!isInternal) return;

      event.preventDefault();
      node.dragOver = true;

      // Auto-expand if collapsed and not loading
      if (!node.expanded && !node.loading) {
        // Clear any existing timeout for this node
        if (this.expandTimeouts.has(node)) {
          clearTimeout(this.expandTimeouts.get(node));
        }
        // Set a new timeout
        const timeout = setTimeout(() => {
          this.toggleExpand(node, true); // expand without navigating
          this.expandTimeouts.delete(node);
        }, 800); // 800ms delay (we need to hover that time to auto expand when dragging a item)
        this.expandTimeouts.set(node, timeout);
      }
    },

    clearAllDragStates() {
      if (!this.isRootInstance) return; // Only root should handle global reset

      const clearNode = (node) => {
        node.dragOver = false;
        if (node.children) {
          node.children.forEach(clearNode);
        }
      };
      this.rootNodes.forEach(clearNode);

      // Clear all pending expand timeouts
      this.expandTimeouts.forEach(timeout => clearTimeout(timeout));
      this.expandTimeouts.clear();
    },

    handleDragLeave(event, node) {
      if (!node.isDir) return;
      // Don't clear drag state if we're just moving to a child element
      if (event.currentTarget.contains(event.relatedTarget)) {
        return;
      }
      node.dragOver = false;
      // Clear auto-expand timeout
      if (this.expandTimeouts.has(node)) {
        clearTimeout(this.expandTimeouts.get(node));
        this.expandTimeouts.delete(node);
      }
    },

    async handleDrop(event, node) {
      if (!node.isDir) return;
      if (!this.canModify) return;
      node.dragOver = false;

      // Clear any pending timeout
      if (this.expandTimeouts.has(node)) {
        clearTimeout(this.expandTimeouts.get(node));
        this.expandTimeouts.delete(node);
      }

      const isInternal = Array.from(event.dataTransfer.types).includes(
        'application/x-filebrowser-internal-drag'
      );
      if (!isInternal) return;

      event.preventDefault();

      // Get selected items
      const selectedIndices = state.selected;
      if (!selectedIndices || selectedIndices.length === 0) return;

      // Build list of items to move
      const items = [];
      for (const index of selectedIndices) {
        const item = state.req.items[index];
        if (!item) continue;

        // Prevent dropping onto itself or into its own subdirectory
        if (item.path === node.path) continue; // cannot drop onto itself
        if (node.path.startsWith(item.path + '/')) continue; // cannot drop into its own subdirectory

        items.push({
          from: item.path,
          fromSource: item.source || state.req.source,
          to: joinPath(node.path, item.name),
          toSource: node.source || this.currentSource,
          name: item.name,
        });
      }

      if (items.length === 0) return;

      // We'll use the same logic as Breadcrumbs
      const normalizePath = (path) => {
        if (!path || path === "/") return "/";
        return path.replace(/\/$/, '');
      };
      const targetDir = normalizePath(node.path);
      const sourceDir = normalizePath(state.req.path);
      if (targetDir === sourceDir) {
        notify.showErrorToast(this.$t("files.sameFolder"));
        return;
      }
      // Fetch destination contents to check for conflicts
      let response;
      try {
        if (this.isShare) {
          response = await resourcesApi.fetchFilesPublic(node.path, this.shareHash);
        } else {
          response = await resourcesApi.fetchFiles(this.currentSource, node.path);
        }
      } catch (err) {
        console.error('Failed to fetch dir for conflict check', err);
        notify.showErrorToast(this.$t('files.cannotAccesDir'));
        return;
      }

      const targetItems = response?.items || [];

      // If any item name already exists in destination
      const conflict = items.some(item => {
        const itemName = item.to.split('/').pop();
        return targetItems.some(target => target.name === itemName);
      });

      const performAction = async (overwrite, rename) => {
        mutations.showHover({
          name: 'move',
          props: { operationInProgress: true },
        });

        try {
          if (this.isShare) {
            await resourcesApi.moveCopyPublic(this.shareHash, items, 'move', overwrite, rename);
          } else {
            await resourcesApi.moveCopy(items, 'move', overwrite, rename);
          }
          const buttonAction = () => {
            if (this.isShare) {
              goToItem(null, node.path, {}, false, this.shareHash);
            } else {
              goToItem(this.currentSource, node.path, {});
            }
          };
          const buttonProps = {
            icon: 'folder',
            buttons: [{
              label: this.$t('buttons.goToItem'),
              primary: true,
              action: buttonAction,
            }]
          };
          notify.showSuccess(this.$t('prompts.moveSuccess'), buttonProps);
          mutations.closeHovers();
          mutations.setReload(true);
        } catch (error) {
          mutations.closeHovers();
          throw error;
        }
      };
      if (conflict) {
        mutations.showHover({
          name: 'replace-rename',
          confirm: async (event, option) => {
            const overwrite = option === 'overwrite';
            const rename = option === 'rename';
            event.preventDefault();
            mutations.closeHovers();
            await performAction(overwrite, rename);
          },
        });
        return;
      }
      await performAction(false, false);
    },

    isSelected(node) {
      const prompt = getters.currentPrompt();
      if (!prompt) return false;

      const props = prompt.props || {};

      // Collect items from various prompt prop
      let candidates = [];

      // Most prompts store items in an "items" array
      if (props.items && Array.isArray(props.items)) {
        candidates = props.items;
      }
      // Rename, share, info have a single "item"
      else if (props.item) {
        candidates = [props.item];
      }
      // Access prompt uses sourceName + path
      else if (props.sourceName && props.path) {
        candidates = [{ source: props.sourceName, path: props.path, isDir: true }];
      }
      // Upload prompt uses targetPath + targetSource (yeah, I wanted to support upload from the tree context menu too)
      else if (props.targetPath && props.targetSource) {
        candidates = [{ source: props.targetSource, path: props.targetPath, isDir: true }];
      }

      if (candidates.length === 0) return false;

      // Compare node to each candidate (usually only one, but safe to loop)
      return candidates.some(selected => {
        if (!selected || !selected.path) return false;
        if (this.isShare) {
          return selected.path === node.path;
        } else {
          return selected.source === node.source && selected.path === node.path;
        }
      });
    }
  },
};
</script>

<style scoped>
.file-tree {
  font-size: 0.9rem;
  user-select: none;
  min-width: 100%;
  width: fit-content;
}

.file-tree-root {
  padding: 0.25em 0;
}

.tree-list {
  list-style: none;
  padding-left: 0;
  margin: 0;
}

/* indent nested lists (children) */
.tree-list .tree-list {
  margin-left: 1.2em; /* this creates the tree effect */
  border-left: 0.05em dotted var(--primaryColor); /* guide line */
  padding-left: 0.2em;
}

.tree-item {
  margin: 0;
  padding: 0;
}

.tree-node {
  display: flex;
  align-items: center;
  padding: 0.15em 0.3em;
  border-radius: 0.4em;
  transition: background-color 0.1s;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
  width: 100%;
  box-sizing: border-box;
}

.tree-node:hover {
  background-color: var(--surfaceSecondary);
}

.tree-node.current-item {
  background-color: var(--primaryColor);
  color: white;
}

.tree-node.drag-over {
  background-color: var(--primaryColor) !important;
  opacity: 0.8;
  outline: 2px solid var(--primaryColor);
  outline-offset: -1px;
}

.expand-icon,
.expand-icon-placeholder {
  display: inline-flex;
  width: 1.5em;
  justify-content: center;
  align-items: center;
  font-size: 1em;
  color: var(--textSecondary);
  flex-shrink: 0;
}

.expand-icon {
  cursor: pointer;
}

.expand-icon:hover {
  color: var(--primaryColor);
}

.tree-node.current-item .expand-icon:hover {
  color: white;
}

.node-icon {
  margin-right: 0.3em;
  font-size: 1.2em;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  width: 1.5em;
  height: 1.5em;
  justify-content: center;
}

.node-icon :deep(i) {
  font-size: 1.2em;
}

.node-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-left: 0.2em;
  min-width: 0;
}

.tree-loading {
  display: flex;
  justify-content: center;
  padding: 0.5em;
  opacity: 0.7;
}

.tree-error {
  padding: 1em;
  text-align: center;
  color: var(--errorColor);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5em;
}

.tree-error.small {
  padding: 0.5em;
  font-size: 0.9em;
}

.tree-empty {
  padding: 0.5em;
  text-align: center;
  color: var(--textSecondary);
  font-style: italic;
}

.tree-empty-folder {
  padding: 0.5em 1em;
  margin-left: 1.2em;
  text-align: center;
  color: var(--textSecondary);
  font-style: italic;
  font-size: 0.85em;
}
</style>