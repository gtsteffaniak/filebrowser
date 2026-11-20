<template>
  <div v-if="items.length > 0" id="breadcrumbs">
    <ul>
      <li>
        <router-link :to="base" :aria-label="t('general.home')" :title="t('general.home')"
          :class="{ 'drag-over': dragOverItem?.type === 'home', 'context-item': isSelected(homeLink) }"
          @dragenter.prevent="dragEnter($event, homeLink)"
          @dragleave.prevent="dragLeave($event, homeLink)"
          @dragover.prevent="dragOver($event, homeLink)"
          @drop.prevent="drop($event, homeLink)"
          @contextmenu.prevent.stop="openMenu($event, homeLink)">
          <i class="material-symbols">home</i>
        </router-link>
      </li>
      <li class="item" v-for="(link, index) in items" :key="index">
        <router-link
          :to="link.url"
          :aria-label="`breadcrumb-link-${link.name}`"
          :title="link.name"
          :class="{
            'drag-over': dragOverItem?.path === link.path,
            'context-item': isSelected(link),
          }"
          @dragenter="dragEnter($event, link)"
          @dragleave="dragLeave($event, link)"
          @dragover="dragOver($event, link)"
          @drop="drop($event, link)"
          @contextmenu.prevent.stop="openMenu($event, link)">
          <span class="breadcrumb-text">{{ link.name }}</span>
        </router-link>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "@/i18n/index.ts";
import { resourcesApi } from "@/api/index.js";
import { notify } from "@/notify/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import { encodedPath, joinPath } from "@/utils/url.js";
import { goToItemNotificationButton } from "@/utils/notificationActions.js";
import { notifyMoveCopyFailure } from "@/utils/appNotifications.js";

type BreadcrumbType = "home" | "normal" | "truncated";

interface BreadcrumbLink {
  name: string;
  url: string;
  path: string;
  type: BreadcrumbType;
}

const INTERNAL_DRAG_TYPE = "application/x-filebrowser-internal-drag";

const { t } = useI18n();

const dragOverItem = ref<BreadcrumbLink | null>(null); // Track which breadcrumb is being dragged over

const base = computed(() =>
  getters.isShare() ? getters.sharePathBase() : `/files/${state.req.source}/`,
);
const isDroppable = computed(() => !!getters.sourcePermissions().modify);

const homeLink = computed<BreadcrumbLink>(() => ({
  name: t("general.home"),
  url: base.value,
  path: "/",
  type: "home",
}));

const items = computed<BreadcrumbLink[]>(() => {
  const req = state.req;
  if (!req.path) return [];
  const originalParts = req.path.split("/");
  const encodedParts = encodedPath(req.path).split("/");
  // Remove empty strings from both arrays
  if (originalParts[0] === "") {
    originalParts.shift();
    encodedParts.shift();
  }
  if (originalParts.at(-1) === "") {
    originalParts.pop();
    encodedParts.pop();
  }
  const breadcrumbs: BreadcrumbLink[] = [];
  let buildRef = base.value;
  let accumulatedPath = "";

  for (const [i, origPart] of originalParts.entries()) {
    buildRef = `${buildRef + (encodedParts.at(i) ?? "")}/`;
    accumulatedPath = accumulatedPath ? `${accumulatedPath}/${origPart}` : origPart;
    breadcrumbs.push({
      name: origPart,
      url: buildRef,
      path: `/${accumulatedPath}`,
      type: "normal",
    });
  }
  if (breadcrumbs.length > 3) {
    breadcrumbs.splice(0, breadcrumbs.length - 4);
    const [first] = breadcrumbs;
    if (first) {
      breadcrumbs[0] = { name: "...", url: first.url, path: first.path, type: "truncated" };
    }
  }
  return breadcrumbs;
});

/** Name of the folder a breadcrumb stands for ("..." and home have no name of their own). */
function linkName(link: BreadcrumbLink, source: string): string {
  if (link.type === "home") return source || link.name;
  if (link.type === "truncated") return link.path.split("/").filter(Boolean).pop() ?? link.name;
  return link.name;
}

interface BreadcrumbSelection {
  source?: string;
  path?: string;
  heldSelected?: boolean;
}

interface PromptProps {
  items?: unknown;
  item?: BreadcrumbSelection;
  sourceName?: string;
  path?: string;
  targetPath?: string;
  targetSource?: string;
}

function selectionKey(source: string | undefined, path: string) {
  return getters.isShare() ? path : `${source ?? ""}\u0000${path}`;
}

const selectedKeys = computed(() => {
  const keys = new Set<string>();
  for (const prompt of state.prompts) {
    const promptProps = (prompt.props || {}) as PromptProps;
    let candidates: BreadcrumbSelection[] = [];
    if (Array.isArray(promptProps.items)) {
      candidates = promptProps.items as BreadcrumbSelection[];
    } else if (promptProps.item) {
      candidates = [promptProps.item];
    } else if (promptProps.sourceName && promptProps.path) {
      candidates = [{ source: promptProps.sourceName, path: promptProps.path }];
    } else if (promptProps.targetPath && promptProps.targetSource) {
      candidates = [{ source: promptProps.targetSource, path: promptProps.targetPath }];
    }
    for (const selected of candidates) {
      if (!selected.path || selected.heldSelected) continue; // picked from ListingView
      keys.add(selectionKey(selected.source, selected.path));
    }
  }
  return keys;
});

function isSelected(link: BreadcrumbLink): boolean {
  return selectedKeys.value.has(selectionKey(state.req.source || state.sources?.current, link.path));
}

function openMenu(event: MouseEvent, link: BreadcrumbLink) {
  if (getters.currentPromptName() === "ContextMenu") return;
  mutations.resetSelected();
  const source = state.req.source || state.sources?.current || "";
  const isCurrent = normalizePath(link.path) === normalizePath(state.req.path);
  mutations.showPrompt({
    name: "ContextMenu",
    props: {
      posX: event.clientX,
      posY: event.clientY,
      breadcrumb: true,
      items: [
        {
          name: linkName(link, source),
          path: link.path,
          url: link.url,
          source,
          type: "directory",
          isDir: true,
          ...(isCurrent && {
            size: state.req.size,
            modified: state.req.modified,
            created: state.req.created,
          }),
        },
      ],
    },
  });
}

onMounted(() => window.addEventListener("dragend", clearDragState));
onBeforeUnmount(() => window.removeEventListener("dragend", clearDragState));

function isInternalDrag(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types ?? []).includes(INTERNAL_DRAG_TYPE);
}

function dragEnter(event: DragEvent, link: BreadcrumbLink) {
  // Don't allow drag over "..." (is a truncated breadcrumb)
  if (!isDroppable.value || link.type === "truncated") return;
  if (!isInternalDrag(event)) return;
  event.preventDefault();
  dragOverItem.value = link;
}

function dragOver(event: DragEvent, link: BreadcrumbLink) {
  if (!isDroppable.value || link.type === "truncated") return;
  if (!isInternalDrag(event)) return;
  event.preventDefault();
}

function dragLeave(event: DragEvent, link: BreadcrumbLink) {
  // Don't clear drag state if we're just moving to a child element
  if ((event.currentTarget as Node | null)?.contains(event.relatedTarget as Node | null)) {
    return;
  }
  if (
    dragOverItem.value?.path === link.path ||
    (dragOverItem.value?.type === "home" && link.type === "home")
  ) {
    clearDragState();
  }
}

function clearDragState() {
  dragOverItem.value = null;
}

interface MoveItem {
  from: string;
  fromSource: string | undefined;
  to: string;
  toSource: string;
  itemType: string | undefined;
  isDir: boolean;
  name: string;
}

// Normalize paths for comparison
function normalizePath(path: string | undefined): string {
  if (!path || path === "/") return "/";
  return path.replace(/\/$/, ""); // Remove trailing slash
}

async function drop(event: DragEvent, link: BreadcrumbLink) {
  // Don't allow drop on "..."
  if (link.type === "truncated") return;
  if (!isDroppable.value) return;

  event.preventDefault();
  event.stopPropagation();
  clearDragState();

  if (!isInternalDrag(event)) return;

  // Get the target path for this breadcrumb
  const targetPath = link.path;
  const currentPath = state.req.path;
  const source = state.req.source ?? "";

  if (normalizePath(targetPath) === normalizePath(currentPath)) {
    notify.showErrorToast(t("files.sameFolder"));
    console.error("Cannot move to same folder");
    return;
  }

  // Build list of items to move from selected items
  let itemsToMove: MoveItem[] = [];
  for (const i of state.selected) {
    if (typeof i !== "number" || i < 0 || i >= state.req.items.length) continue;

    const selectedItem = state.req.items.at(i);
    if (!selectedItem) continue;

    const fromPath = selectedItem.path || joinPath(state.req.path, selectedItem.name);

    itemsToMove.push({
      from: fromPath,
      fromSource: selectedItem.source,
      to: joinPath(targetPath, selectedItem.name),
      toSource: source,
      itemType: selectedItem.type,
      isDir: selectedItem.type === "directory",
      name: selectedItem.name,
    });
  }

  // Filter out invalid moves
  itemsToMove = itemsToMove.filter((item) => {
    if (item.from === item.to) return false;

    // Prevent moving a directory into itself -- likely never will happen but just in case
    if (item.itemType === "directory") {
      const fromDir = normalizePath(item.from);
      const toDir = normalizePath(item.to);
      // Check if destination is inside the directory
      if (toDir.startsWith(`${fromDir}/`)) {
        return false;
      }
    }
    return true;
  });

  // Check for conflicts in target directory
  let targetDirItems: { name: string }[] = [];
  try {
    if (getters.isShare()) {
      const response = await resourcesApi.fetchFilesPublic(targetPath, state.shareInfo.hash);
      targetDirItems = response?.items ?? [];
    } else {
      const response = await resourcesApi.fetchFiles(source, targetPath);
      targetDirItems = response?.items ?? [];
    }
  } catch (_e) {
    notify.showError(t("files.cannotAccesDir"));
    return;
  }

  // if any item conflics will show replace-rename prompt later
  const conflict = itemsToMove.some((item) => {
    const itemName = item.to.split("/").pop(); // Extract filename from destination path
    return targetDirItems.some((targetItem) => targetItem.name === itemName);
  });

  const moveAction = async (overwrite: boolean, rename: boolean) => {
    mutations.showPrompt({
      name: "move",
      props: { operationInProgress: true },
    });

    try {
      if (getters.isShare()) {
        await resourcesApi.moveCopyPublic(state.shareInfo.hash, itemsToMove, "move", overwrite, rename);
      } else {
        await resourcesApi.moveCopy(itemsToMove, "move", overwrite, rename);
      }
      notify.showSuccess(t("prompts.moveSuccess"), {
        icon: "folder",
        buttons: [
          goToItemNotificationButton(t("buttons.goToItem"), source, targetPath, getters.isShare()),
        ],
      });
      mutations.closeTopPrompt();
      mutations.setReload(true);
    } catch (error) {
      mutations.closeTopPrompt();
      notifyMoveCopyFailure(error);
    }
  };
  if (conflict) {
    mutations.showPrompt({
      name: "replace-rename",
      pinned: true,
      confirm: (e: Event, option: string) => {
        const overwrite = option === "overwrite";
        const rename = option === "rename";
        e.preventDefault();
        mutations.closeTopPrompt();
        void moveAction(overwrite, rename);
      },
    });
    return;
  }
  try {
    await moveAction(false, false);
  } catch (error) {
    notifyMoveCopyFailure(error);
  }
}
</script>

<style scoped>
#breadcrumbs {
  overflow-x: auto;
  scrollbar-width: none;
}

#breadcrumbs::-webkit-scrollbar {
  display: none;
}

#breadcrumbs * {
  box-sizing: unset;
}

#breadcrumbs ul {
  display: flex;
  margin: 0;
  padding: 0;
  margin-bottom: 0.5em;
}

#breadcrumbs ul li {
  display: inline-block;
  position: relative;
}

#breadcrumbs ul li a {
  display: flex;
  height: 0.85em;
  background: var(--surfacePrimary);
  text-align: center;
  padding: 0.85em;
  padding-right: 1.6em;
  padding-left: 1.7em;
  position: relative;
  text-decoration: none;
  color: var(--textPrimary);
  border-radius: 0;
  align-content: center;
  align-items: center;
  transition: background-color 0.2s ease, color 0.2s ease, padding-left 0.2s ease,
    padding-right 0.2s ease, margin-right 0.2s ease, border-radius 0.2s ease, clip-path 0.2s ease;
  user-select: none;
  white-space: nowrap;
  max-width: 90vw;
  --chevron-depth: calc(1.275em + 1px);
  --has-left-notch: 1;
  --has-right-point: 1;
  clip-path: polygon(
    0 0,
    calc(100% - var(--has-right-point) * var(--chevron-depth)) 0,
    100% 50%,
    calc(100% - var(--has-right-point) * var(--chevron-depth)) 100%,
    0 100%,
    calc(var(--has-left-notch) * var(--chevron-depth)) 50%);
  margin-right: -1em;
}

#breadcrumbs ul li a .breadcrumb-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

#breadcrumbs ul li:first-child a {
  border-top-left-radius: 0.85em;
  border-bottom-left-radius: 0.85em;
  --has-left-notch: 0;
  padding-left: 1.1em;
}

#breadcrumbs ul li:last-child a {
  padding-right: 1.275em;
  border-top-right-radius: 0.85em;
  border-bottom-right-radius: 0.85em;
  --has-right-point: 0;
  margin-right: 0;
}

#breadcrumbs ul li a:hover {
  background: var(--primaryColor);
  color: white;
}

#breadcrumbs ul li a.context-item {
  --highlight-border: var(--borderWidth);
  background: var(--primaryColor);
  color: white;
}

#breadcrumbs ul li a.context-item::before {
  content: "";
  position: absolute;
  z-index: -1;
  inset: calc(var(--highlight-border) - 1px);
  background: color-mix(in srgb, var(--primaryColor) 50%, var(--surfacePrimary));
  clip-path: polygon(
    calc(var(--has-left-notch) * 1.414 * var(--highlight-border)) 0,
    calc(100% - var(--has-right-point) * (var(--chevron-depth) - 0.586 * var(--highlight-border))) 0,
    calc(100% - var(--has-right-point) * 0.414 * var(--highlight-border)) 50%,
    calc(100% - var(--has-right-point) * (var(--chevron-depth) - 0.586 * var(--highlight-border))) 100%,
    calc(var(--has-left-notch) * 1.414 * var(--highlight-border)) 100%,
    calc(var(--has-left-notch) * (var(--chevron-depth) + 0.414 * var(--highlight-border))) 50%);
}

#breadcrumbs ul li:first-child a.context-item::before {
  border-radius: calc(0.85em - var(--highlight-border)) 0 0 calc(0.85em - var(--highlight-border));
}

#breadcrumbs ul li:last-child a.context-item::before {
  border-radius: 0 calc(0.85em - var(--highlight-border)) calc(0.85em - var(--highlight-border)) 0;
}

.drag-over {
  background: var(--primaryColor) !important; /* Needs !important to make the hover effect work when dragging items */
  color: white !important;
  z-index: 2;
  animation: breadcrumb-pulse 0.5s ease-in-out infinite;
}

@keyframes breadcrumb-pulse {
  0% { transform: scale(1); }
  50% { transform: scale(1.05); }
  100% { transform: scale(1); }
}
</style>
