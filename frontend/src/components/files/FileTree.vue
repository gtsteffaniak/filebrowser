<template>
  <div ref="treeEl" class="file-tree" :class="{ 'file-tree-root': isRootInstance }">
    <!-- Loading root -->
    <div v-if="isRootInstance && loading" class="tree-loading">
      <LoadingSpinner size="small" mode="placeholder" />
    </div>
    <!-- Error state -->
    <div v-if="isRootInstance && error" class="tree-error">
      <i class="material-symbols">error</i>
      <span>{{ $t("prompts.error") }}</span>
    </div>
    <ul v-if="effectiveNodes?.length" class="tree-list">
      <li v-for="node in effectiveNodes" :key="node.path" class="tree-item">
        <a
          :href="getNodeUrl(node)"
          class="tree-node"
          :class="{
            'current-item': isCurrentItem(node),
            'has-children': node.childrenCount > 0,
            'drag-over': node.dragOver,
            'context-item': isSelected(node),
            'hidden-file': node.isHidden,
          }"
          @click="onNodeClick($event, node)"
          @contextmenu.prevent="handleContextMenu($event, node)"
          @dragover.prevent="handleDragOver($event, node)"
          @dragleave.prevent="handleDragLeave($event, node)"
          @drop.prevent="handleDrop($event, node)"
        >
          <!-- Expand/collapse arrow (only for folders) -->
          <span
            v-if="node.isDir"
            class="expand-icon"
            @click.stop.prevent="toggleExpand(node, true)"
          >
            <i class="material-symbols" :class="{ rotated: node.expanded }">
              chevron_right <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            </i>
          </span>
          <span v-else class="expand-icon-placeholder"></span>
          <Icon
            :mimetype="node.isDir ? 'directory' : (node.type ?? '')"
            :filename="node.name"
            :isShared="node.isShared"
            class="node-icon"
          />
          <i v-if="node.pinned" class="material-symbols pinned-indicator">push_pin</i>
          <span class="node-name" :title="node.name">
            {{ node.name }}
          </span>
        </a>
        <!-- Recursive children if expanded -->
        <FileTree
          v-if="node.expanded && node.children && node.children.length > 0"
          :nodes="node.children"
          :currentSource="treeSource"
          :shareHash="shareHash"
          :currentPath="currentPath"
        />
        <div v-else-if="node.expanded && node.children && node.children.length === 0" class="tree-empty-folder">
          <i class="material-symbols-outlined">sentiment_dissatisfied</i>
          <span>{{ $t('files.lonely') }}</span>
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
      <i class="material-symbols-outlined">sentiment_dissatisfied</i>
      {{ $t('files.lonely') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { resourcesApi } from '@/api';
import Icon from '@/components/files/Icon.vue';
import LoadingSpinner from '@/components/LoadingSpinner.vue';
import i18n from '@/i18n/index.ts';
import { state, getters, mutations } from '@/store';
import { eventBus } from '@/store/eventBus';
import type { FileListItem } from '@/store/types.ts';
import { buildItemUrl, goToItem, joinPath } from '@/utils/url';
import { goToItemNotificationButton } from '@/utils/notificationActions';
import { notify } from '@/notify';
import { notifyMoveCopyFailure } from '@/utils/appNotifications';
import { getObjectProperty } from '@/utils/object.js';
import { sortedItems } from '@/utils/sort.ts';

export interface TreeNode extends FileListItem {
  isDir: boolean;
  expanded: boolean;
  loading: boolean;
  children: TreeNode[] | null;
  childrenCount: number;
  childrenError: string | null;
  dragOver: boolean;
  isHidden?: boolean;
}

interface TreeScope {
  source: string | null;
  hash: string | null;
  rootPath: string;
}

interface MoveItem {
  from: string;
  fromSource?: string;
  to: string;
  toSource?: string;
  name: string;
  isDir?: boolean;
}

interface SelectionCandidate {
  source?: string;
  path?: string;
  isDir?: boolean;
  heldSelected?: boolean;
}

interface PromptProps {
  items?: unknown;
  item?: SelectionCandidate;
  sourceName?: string;
  path?: string;
  targetPath?: string;
  targetSource?: string;
}

const props = withDefaults(
  defineProps<{
    // Will be ignored if we are in a share
    currentSource?: string | null;
    // Nodes to render (for recursion, root uses its own data and leaves this null)
    // with 'root', I mean whole tree view component, the one that contains all the nodes/folders.
    nodes?: TreeNode[] | null;
    currentPath?: string;
    // Root path to start from
    rootPath?: string;
  }>(),
  {
    currentSource: null,
    nodes: null,
    currentPath: '/',
    rootPath: '/',
  }
);

const t = i18n.global.t.bind(i18n.global);

const treeEl = ref<HTMLElement | null>(null);
const rootNodes = ref<TreeNode[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);
const expanding = ref(false);
const isRefreshing = ref(false);
let refreshQueued = false;
let unwatchReload: (() => void) | null = null;
const expandTimeouts = new Map<TreeNode, ReturnType<typeof setTimeout>>(); // For auto-expand when dragging a item to a folder
const pendingExpandPath = ref<string | null>(null);
const rootLoaded = ref(false);

const treeSource = computed(() => props.currentSource || state.sources.current || null);
const shareHash = computed(() => getters.shareHash() || null);
const isShare = computed(() => !!shareHash.value);
// Determine if this instance is the root of the tree
const isRootInstance = computed(() => props.nodes === null);
const effectiveNodes = computed(() => (isRootInstance.value ? rootNodes.value : props.nodes));
const canModify = computed(() => getters.sourcePermissions().modify);
// Defaults to true
const showFiles = computed(() => !state.user?.hideFilesInTree);
const normalizedCurrentPath = computed(() => normalizePath(props.currentPath));

async function loadRoot() {
  if (expanding.value) return;
  error.value = null;
  rootLoaded.value = false;
  if ((!treeSource.value && !shareHash.value) || !isRootInstance.value) {
    rootNodes.value = [];
    return;
  }
  loading.value = true;
  try {
    const items = await fetchItems(props.rootPath);
    rootNodes.value = items.map(item => createNode(item));
    rootLoaded.value = true;
  } catch (err) {
    console.error('Failed to load tree root:', err);
    error.value = (err as Error).message || 'Failed to load';
    rootNodes.value = [];
  } finally {
    loading.value = false;
  }
}

function captureScope(): TreeScope {
  return { source: treeSource.value, hash: shareHash.value, rootPath: props.rootPath };
}

function isCurrentScope(scope: TreeScope) {
  return scope.source === treeSource.value && scope.hash === shareHash.value && scope.rootPath === props.rootPath;
}

async function fetchItems(path: string, scope: TreeScope = captureScope()): Promise<FileListItem[]> {
  let items: FileListItem[];
  if (scope.hash) {
    const res = await resourcesApi.fetchFilesPublic(path, scope.hash, '', false, false, true);
    items = res.items || [];
  } else {
    const res = await resourcesApi.fetchFiles(scope.source, path, false, false, true);
    items = res.items || [];
  }
  // Filter out files if showFiles is false
  if (!showFiles.value) {
    items = items.filter(item => item.type === 'directory');
  }
  const dirs = items.filter(item => item.type === 'directory');
  const files = items.filter(item => item.type !== 'directory');
  return [...sortedItems(dirs, 'name', true), ...sortedItems(files, 'name', true)];
}

function createNode(item: FileListItem): TreeNode {
  return {
    source: treeSource.value || undefined,
    type: item.type,
    isDir: item.type === 'directory',
    expanded: false,
    loading: false,
    children: null,
    childrenCount: 0,
    childrenError: null,
    isShared: item.isShared,
    dragOver: false, // For drag highlight
    isHidden: item.hidden,
    ...item
  };
}

async function toggleExpand(node: TreeNode, skipNavigate = false) {
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
      const items = await fetchItems(node.path);
      node.children = items.map(item => createNode(item));
      node.childrenCount = node.children.length;
    } catch (err) {
      console.error('Failed to load children for', node.path, err);
      node.children = [];
      node.childrenCount = 0;
      node.childrenError = (err as Error).message || 'Failed to load';
    } finally {
      node.loading = false;
    }
  }

  if (!skipNavigate && node.path !== props.currentPath) {
    navigateTo(node);
  }
}

async function tryExpandNodes() {
  if (expanding.value || !rootLoaded.value || !pendingExpandPath.value) return;
  const target = pendingExpandPath.value;
  await expandToPath(target);
  if (pendingExpandPath.value === target) {
    pendingExpandPath.value = null;
  } else {
    void tryExpandNodes();
  }
}

function getNodeUrl(node: TreeNode) {
  const source = node.source || treeSource.value;
  return buildItemUrl(source, node.path, true);
}

function onNodeClick(event: MouseEvent, node: TreeNode) {
  if (event.button !== 0) {
    return;
  }
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
    return;
  }
  event.preventDefault();
  handleNodeClick(node);
}

function handleNodeClick(node: TreeNode) {
  // don't navigate to current item if we're already viewing it
  if (isCurrentItem(node)) {
    if (node.isDir) {
      void toggleExpand(node, false); // If the item is a folder, just expand/collapse instead of navigate again
    }
    return;
  }
  if (node.isDir) {
    navigateTo(node);
  } else {
    navigateTo(node);
  }
}

function handleContextMenu(event: MouseEvent, node: TreeNode) {
  event.preventDefault();
  event.stopPropagation();

  // Build an array with the single node (we don't support multi-select in tree yet, maybe later! But I'm not sure tbh)
  const items = [node];
  mutations.showPrompt({
    name: 'ContextMenu',
    props: {
      posX: event.clientX,
      posY: event.clientY,
      items: items, // Pass to context menu
    },
  });
}

function navigateTo(node: TreeNode) {
  if (isShare.value) {
    mutations.setNavigationTransitioning(true);
    goToItem(shareHash.value, node.path, {}, false, true);
  } else {
    mutations.setNavigationTransitioning(true);
    goToItem(treeSource.value, node.path, {});
  }
}

function normalizePath(p?: string | null) {
  if (!p || p === '/') return '/';
  return p.replace(/\/+$/, '');
}

function isCurrentItem(node: TreeNode) {
  const normalizedNodePath = normalizePath(node.path);
  if (isShare.value) {
    return normalizedNodePath === normalizedCurrentPath.value;
  } else {
    return node.source === treeSource.value && normalizedNodePath === normalizedCurrentPath.value;
  }
}

async function expandToPath(targetPath: string) {
  if (!targetPath || targetPath === '/' || expanding.value) return;
  expanding.value = true;
  try {
    if (isRootInstance.value && !rootNodes.value.length) {
      await loadRoot();
    }
    const pathParts = targetPath.split('/').filter(p => p);
    if (pathParts.length === 0) return;

    let currentLevel = (isRootInstance.value ? rootNodes.value : props.nodes) ?? [];

    for (const part of pathParts) {
      const node = currentLevel.find(n => n.name === part && n.isDir);
      if (!node) break;

      if (!node.expanded) {
        await toggleExpand(node, true);
      }
      currentLevel = node.children || [];
    }

    await nextTick();
    scrollToCurrent();
  } catch (err) {
    console.error('Error expanding path:', err);
  } finally {
    expanding.value = false;
  }
}

function scrollToTop() {
  let el: HTMLElement | null = treeEl.value?.parentElement ?? null;
  while (el) {
    if (el.scrollTop > 0 && /(auto|scroll)/.test(getComputedStyle(el).overflowY)) {
      el.scrollTo({ top: 0, behavior: 'smooth' });
      return;
    }
    el = el.parentElement;
  }
}

function scrollToCurrent() {
  if (!treeEl.value) return;
  const current = treeEl.value.querySelector('.tree-node.current-item');
  if (current) {
    current.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }
}

async function refresh() {
  if (!isRootInstance.value) return;
  if (isRefreshing.value) {
    refreshQueued = true;
    return;
  }
  if (!treeSource.value && !shareHash.value) return;
  isRefreshing.value = true;
  for (const timeout of expandTimeouts.values()) {
    clearTimeout(timeout);
  }
  expandTimeouts.clear();
  const scope = captureScope();
  try {
    const nodes = await syncNodes(rootNodes.value, props.rootPath, scope);
    if (!isCurrentScope(scope)) return;
    if (!sameNodeList(rootNodes.value, nodes)) {
      rootNodes.value = nodes;
    }
    error.value = null;
    rootLoaded.value = true;
    void tryExpandNodes();
  } catch (err) {
    console.error('Failed to refresh tree:', err);
  } finally {
    isRefreshing.value = false;
    if (refreshQueued) {
      refreshQueued = false;
      void refresh();
    }
  }
}

async function syncNodes(existing: TreeNode[], path: string, scope: TreeScope): Promise<TreeNode[]> {
  const items = await fetchItems(path, scope);
  if (!isCurrentScope(scope)) return existing;
  const known = new Map(existing.map((node) => [node.path, node]));
  const merged = items.map((item) => {
    const currentNode = known.get(item.path);
    if (!currentNode) return createNode(item);
    if (
      (Object.keys(item) as (keyof FileListItem)[]).every((key) => currentNode[key] === item[key]) &&
      currentNode.isHidden === item.hidden &&
      !!currentNode.pinned === !!item.pinned &&
      !!currentNode.isShared === !!item.isShared
    ) {
      return currentNode;
    }
    const wasDir = currentNode.isDir;
    const node: TreeNode = {
      ...createNode(item),
      expanded: currentNode.expanded,
      loading: currentNode.loading,
      children: currentNode.children,
      childrenCount: currentNode.childrenCount,
      childrenError: currentNode.childrenError,
      dragOver: currentNode.dragOver,
    };
    if (wasDir !== node.isDir) {
      node.expanded = false;
      node.loading = false;
      node.children = null;
      node.childrenCount = 0;
      node.childrenError = null;
    }
    return node;
  });
  for (const node of merged) {
    if (node.isDir && !node.expanded && !node.loading && node.children) {
      node.children = null;
      node.childrenCount = 0;
      node.childrenError = null;
    }
  }
  await Promise.all(
    merged
      .filter((node) => node.isDir && node.expanded && node.children)
      .map(async (node) => {
        try {
          const previousChildren = node.children as TreeNode[];
          const children = await syncNodes(previousChildren, node.path, scope);
          if (!isCurrentScope(scope)) return;
          if (!sameNodeList(previousChildren, children)) {
            node.children = children;
          }
          node.childrenCount = children.length;
          node.childrenError = null;
        } catch (err) {
          console.error('Failed to refresh children for', node.path, err);
        }
      })
  );
  if (!isCurrentScope(scope)) return existing;
  return merged;
}

function sameNodeList(a: TreeNode[], b: TreeNode[]) {
  return a.length === b.length && a.every((node, i) => node === b[i]);
}

function handleDragOver(event: DragEvent, node: TreeNode) {
  if (!node.isDir || !canModify.value) return;

  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    'application/x-filebrowser-internal-drag'
  );
  if (!isInternal) return;

  event.preventDefault();
  node.dragOver = true;

  // Auto-expand if collapsed and not loading
  if (!node.expanded && !node.loading) {
    // Clear any existing timeout for this node
    if (expandTimeouts.has(node)) {
      clearTimeout(expandTimeouts.get(node));
    }
    // Set a new timeout
    const timeout = setTimeout(() => {
      void toggleExpand(node, true); // expand without navigating
      expandTimeouts.delete(node);
    }, 800); // 800ms delay (we need to hover that time to auto expand when dragging a item)
    expandTimeouts.set(node, timeout);
  }
}

function clearAllDragStates() {
  if (!isRootInstance.value) return; // Only root should handle global reset

  const clearNode = (node: TreeNode) => {
    node.dragOver = false;
    if (node.children) {
      node.children.forEach(clearNode);
    }
  };
  rootNodes.value.forEach(clearNode);

  // Clear all pending expand timeouts
  for (const timeout of expandTimeouts.values()) {
    clearTimeout(timeout);
  }
  expandTimeouts.clear();
}

function handleDragLeave(event: DragEvent, node: TreeNode) {
  if (!node.isDir) return;
  // Don't clear drag state if we're just moving to a child element
  if ((event.currentTarget as Node).contains(event.relatedTarget as Node | null)) {
    return;
  }
  node.dragOver = false;
  // Clear auto-expand timeout
  if (expandTimeouts.has(node)) {
    clearTimeout(expandTimeouts.get(node));
    expandTimeouts.delete(node);
  }
}

async function handleDrop(event: DragEvent, node: TreeNode) {
  if (!node.isDir) return;
  if (!canModify.value) return;
  node.dragOver = false;

  // Clear any pending timeout
  if (expandTimeouts.has(node)) {
    clearTimeout(expandTimeouts.get(node));
    expandTimeouts.delete(node);
  }

  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    'application/x-filebrowser-internal-drag'
  );
  if (!isInternal) return;

  event.preventDefault();

  // Get selected items
  const selectedIndices = state.selected;
  if (!selectedIndices || selectedIndices.length === 0) return;

  // Build list of items to move
  const items: MoveItem[] = [];
  for (const index of selectedIndices) {
    const item = getObjectProperty(state.req?.items, index) as FileListItem | undefined;
    if (!item) continue;

    // Prevent dropping onto itself or into its own subdirectory
    if (item.path === node.path) continue; // cannot drop onto itself
    if (node.path.startsWith(`${item.path}/`)) continue; // cannot drop into its own subdirectory

    items.push({
      from: item.path,
      fromSource: item.source || state.req.source,
      to: joinPath(node.path, item.name),
      toSource: node.source || treeSource.value || undefined,
      name: item.name,
      isDir: item.type === 'directory',
    });
  }

  if (items.length === 0) return;

  const targetDir = normalizePath(node.path);
  const sourceDir = normalizePath(state.req.path);
  if (targetDir === sourceDir) {
    notify.showErrorToast(t("files.sameFolder"));
    return;
  }
  // Fetch destination contents to check for conflicts
  let response: { items?: FileListItem[] } | undefined;
  try {
    if (isShare.value) {
      response = await resourcesApi.fetchFilesPublic(node.path, shareHash.value);
    } else {
      response = await resourcesApi.fetchFiles(treeSource.value, node.path);
    }
  } catch (err) {
    console.error('Failed to fetch dir for conflict check', err);
    notify.showErrorToast(t('files.cannotAccesDir'));
    return;
  }

  const targetItems: FileListItem[] = response?.items || [];

  // If any item name already exists in destination
  const conflict = items.some(item => {
    const itemName = item.to.split('/').pop();
    return targetItems.some(target => target.name === itemName);
  });

  const performAction = async (overwrite: boolean, rename: boolean) => {
    mutations.showPrompt({
      name: 'move',
      props: { operationInProgress: true },
    });

    try {
      if (isShare.value) {
        await resourcesApi.moveCopyPublic(shareHash.value, items, 'move', overwrite, rename);
      } else {
        await resourcesApi.moveCopy(items, 'move', overwrite, rename);
      }
      const buttonProps = {
        icon: 'folder',
        buttons: [
          goToItemNotificationButton(
            t('buttons.goToItem'),
            isShare.value ? shareHash.value : treeSource.value,
            node.path,
            isShare.value
          ),
        ],
      };
      notify.showSuccess(t('prompts.moveSuccess'), buttonProps);
      mutations.closeTopPrompt();
      mutations.setReload(true);
    } catch (err) {
      mutations.closeTopPrompt();
      notifyMoveCopyFailure(err);
    }
  };
  if (conflict) {
    mutations.showPrompt({
      name: 'replace-rename',
      pinned: true,
      confirm: async (event: Event, option: string) => {
        const overwrite = option === 'overwrite';
        const rename = option === 'rename';
        event.preventDefault();
        mutations.closeTopPrompt();
        try {
          await performAction(overwrite, rename);
        } catch (err) {
          notifyMoveCopyFailure(err);
        }
      },
    });
    return;
  }
  try {
    await performAction(false, false);
  } catch (err) {
    notifyMoveCopyFailure(err);
  }
}

function selectionKey(source: string | undefined, path: string) {
  return isShare.value ? path : `${source ?? ''}\u0000${path}`;
}

const selectedKeys = computed(() => {
  const keys = new Set<string>();
  for (const prompt of state.prompts) {
    const promptProps = (prompt.props || {}) as PromptProps;

    // Collect items from various prompt prop
    let candidates: SelectionCandidate[] = [];

    // Most prompts store items in an "items" array
    if (promptProps.items && Array.isArray(promptProps.items)) {
      candidates = promptProps.items as SelectionCandidate[];
    }
    // Rename, share, info have a single "item"
    else if (promptProps.item) {
      candidates = [promptProps.item];
    }
    // Access prompt uses sourceName + path
    else if (promptProps.sourceName && promptProps.path) {
      candidates = [{ source: promptProps.sourceName, path: promptProps.path, isDir: true }];
    }
    // Upload prompt uses targetPath + targetSource (yeah, I wanted to support upload from the tree context menu too)
    else if (promptProps.targetPath && promptProps.targetSource) {
      candidates = [{ source: promptProps.targetSource, path: promptProps.targetPath, isDir: true }];
    }

    for (const selected of candidates) {
      if (!selected.path || selected.heldSelected) continue; // picked from ListingView
      keys.add(selectionKey(selected.source, selected.path));
    }
  }
  return keys;
});

function isSelected(node: TreeNode) {
  return selectedKeys.value.has(selectionKey(node.source, node.path));
}

watch(treeSource, async (newVal) => {
  if (isRootInstance.value && newVal) {
    await loadRoot();
    rootLoaded.value = true;
    void tryExpandNodes();
  }
}, { immediate: true });

watch(shareHash, async (newVal) => {
  if (isRootInstance.value && newVal) {
    await loadRoot();
    rootLoaded.value = true;
    void tryExpandNodes();
  }
}, { immediate: true });

watch(() => props.rootPath, (newVal, oldVal) => {
  if (isRootInstance.value && newVal !== oldVal) void loadRoot();
});

watch(() => props.currentPath, (newPath) => {
  if (!isRootInstance.value) return;
  if (newPath !== '/') {
    pendingExpandPath.value = newPath;
    void tryExpandNodes();
  } else {
    void nextTick(scrollToTop);
  }
}, { immediate: true });

watch(showFiles, () => {
  if (isRootInstance.value) {
    void refresh();
  }
});

onMounted(() => {
  // Only the root handles global events, child instances don't need them
  if (!isRootInstance.value) return;
  unwatchReload = watch(
    () => state.reload,
    (newVal) => {
      if (newVal === true) {
        void refresh();
      }
    },
    { flush: 'sync' }
  );
  eventBus.on('itemsDeleted', refresh);
  eventBus.on('itemsRenamed', refresh);
  eventBus.on('itemsMoved', refresh);
  window.addEventListener('dragend', clearAllDragStates);
});

onBeforeUnmount(() => {
  // Clear all expand timeouts
  for (const timeout of expandTimeouts.values()) {
    clearTimeout(timeout);
  }
  expandTimeouts.clear();
  refreshQueued = false;
  if (!isRootInstance.value) return;
  if (unwatchReload) {
    unwatchReload();
  }
  eventBus.off('itemsDeleted', refresh);
  eventBus.off('itemsRenamed', refresh);
  eventBus.off('itemsMoved', refresh);
  window.removeEventListener('dragend', clearAllDragStates);
});
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
  position: relative;
  text-decoration: none;
  color: inherit;
}

.tree-node.hidden-file {
  opacity: 0.5;
}

.tree-node:hover {
  background-color: var(--surfaceSecondary);
}

.tree-node.current-item {
  background-color: var(--primaryColor);
  color: white;
}

.tree-node.context-item,
.tree-node.drag-over {
  background-color: color-mix(in srgb, var(--primaryColor) 25%, transparent) !important;
  color: var(--textPrimary);
}

.expand-icon {
  cursor: pointer;
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

.expand-icon i {
  transition: transform 0.12s ease;
}

.expand-icon i.rotated {
  transform: rotate(90deg);
}

.expand-icon:hover {
  color: var(--primaryColor);
}

.pinned-indicator {
  font-size: 0.9rem !important;
  flex-shrink: 0;
  margin: 0;
}

.tree-node.current-item .expand-icon:hover,
.tree-node.current-item .pinned-indicator {
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
  display: flex;
  align-items: center;
  gap: 0.3em;
  padding: 0.5em 1em;
  margin-left: 1.2em;
  color: var(--textSecondary);
  font-style: italic;
  font-size: 0.9em;
}

.tree-empty-folder i {
  font-size: 1.4em;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  height: 1em;
}

</style>
