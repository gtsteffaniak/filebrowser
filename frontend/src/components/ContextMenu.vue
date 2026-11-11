<template>
  <transition :css="false" @enter="popEnter" @leave="popLeave">
    <div
      v-if="showContext"
      id="context-menu"
      :key="menuKey"
      :style="centered ? undefined : menuPosition"
      class="no-select floating-window"
      :class="{ centered }"
    >
      <template v-if="showNewFileTemplate">
        <div class="context-menu-header">
          <button
            type="button"
            class="action button clickable context-menu-create-toggle"
            aria-label="Back"
            tabindex="0"
            @click="showNewFileTemplate = false"
          >
            <i class="material-symbols">arrow_back</i>
          </button>
        </div>
        <hr class="divider">
        <p v-if="newFileTemplates.length === 0 && isLoggedIn" class="context-menu-empty-message">
          {{ $t('prompts.noTemplate') }}
        </p>
        <button
          v-for="(item, index) in newFileTemplateItems"
          :key="`${index}-${item.name}`"
          type="button"
          class="action no-select"
          :aria-label="item.name"
          :title="item.name"
          @click="createFromTemplate(item.name)"
        >
          <i :class="actionIconClass(item.icon, item.iconClass)">{{ item.icon }}</i>
          <span>{{ item.name }}</span>
        </button>
        <button
          type="button"
          class="action no-select"
          :aria-label="$t('files.emptyFile')"
          :title="$t('files.emptyFile')"
          @click="createEmptyFile"
        >
          <i :class="actionIconClass('insert_drive_file')">insert_drive_file</i>
          <span>{{ $t('files.emptyFile') }}</span>
        </button>
      </template>
      <template v-else>
        <div v-if="showCreateToggle" class="context-menu-header">
          <button
            type="button"
            class="action button clickable context-menu-create-toggle"
            aria-label="Create Actions Toggle"
            :class="{ 'context-menu-create-toggle--disabled': createToggleDisabled }"
            :aria-disabled="createToggleDisabled ? 'true' : 'false'"
            :tabindex="createToggleDisabled ? -1 : 0"
            @click="onCreateToggleClick"
            @mouseenter="onCreateToggleMouseEnter"
            @mouseleave="hideInteractiveTooltip()"
          >
            <i v-if="!showCreate" class="material-symbols">add</i>
            <i v-else class="material-symbols">arrow_back</i>
          </button>
          <div
            v-if="selectedCount > 0"
            @mouseleave="hideInteractiveTooltip()"
            @mouseenter="showHoverTooltip($t('buttons.selectedCount'), $event)"
            class="button selected-count-header"
            :class="{ 'selected-count-header--circle': selectedCount <= 99 }"
          >
            <span>{{ selectedCount }}</span>
          </div>
        </div>
        <hr v-if="showDivider" class="divider">
        <button
          v-for="action in mainActions"
          :key="action.id"
          type="button"
          class="action no-select"
          :aria-label="action.label"
          :title="action.label"
          @click="action.run()"
        >
          <i :class="actionIconClass(action.icon)">{{ action.icon }}</i>
          <span>{{ action.label }}</span>
        </button>
      </template>
    </div>
    <div
      v-else-if="showOverflow"
      id="context-menu"
      key="overflow"
      class="no-select floating-window overflow-menu"
    >
      <button
        v-for="action in overflowActions"
        :key="action.id"
        type="button"
        class="action no-select"
        :aria-label="action.label"
        :title="action.label"
        @click="action.run()"
      >
        <i :class="actionIconClass(action.icon)">{{ action.icon }}</i>
        <span>{{ action.label }}</span>
      </button>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed, markRaw, onBeforeUnmount, onMounted, ref, toRaw, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { resourcesApi, shareApi, usersApi } from "@/api";
import { notify } from "@/notify";
import { getters, mutations, state } from "@/store/index.js";
import type { FileListItem, ReqObject } from "@/store/types.ts";
import { hideInteractiveTooltip, showHoverTooltip } from "@/utils/tooltipHelp.js";
import { goToItem, getParentDir, removeLastDir } from "@/utils/url.js";
import { copyToClipboard } from "@/utils/clipboard";
import downloadFiles from "@/utils/download";
import { canNativeShare, nativeShareFile } from "@/utils/nativeShare";
import { isRichTextPreviewMimeType, getTypeInfoFromExt } from "@/utils/mimetype";
import { isMediaRequest } from "@/utils/mediaFile";
import { actionIconClass } from "@/components/Action.vue";
import { popEnter, popLeave } from "@/utils/transitions";

type MenuItem = FileListItem & { isDir?: boolean; from?: string };

interface MenuAction {
  id: string;
  icon: string;
  label: string;
  run: () => void | Promise<void>;
}

const props = withDefaults(
  defineProps<{
    createOnly?: boolean;
    showCentered?: boolean;
    showLimitedOptions?: boolean;
    items?: MenuItem[] | null;
  }>(),
  {
    createOnly: false,
    showCentered: false,
    showLimitedOptions: false,
    items: null,
  },
);

defineOptions({ inheritAttrs: false });

function isArchivePath(pathOrName?: string): boolean {
  if (!pathOrName || typeof pathOrName !== "string") return false;
  const lower = pathOrName.toLowerCase();
  return lower.endsWith(".zip") || lower.endsWith(".tar.gz") || lower.endsWith(".tgz");
}

function isFolderItem(item: MenuItem | null | undefined): boolean {
  if (!item) return false;
  return !!item.isDir || item.type === "directory";
}

const { t } = useI18n();
const router = useRouter();

const posX = ref(0);
const posY = ref(0);
const showCreate = ref(false);
const showNewFileTemplate = ref(false);

const providedItems = computed<MenuItem[]>(() => {
  // Only a non-empty explicit `items` overrides the selection ([] means "use global state").
  if (Array.isArray(props.items) && props.items.length > 0) {
    return props.items;
  }
  const selected = state.selected;
  if (selected.length === 0) return [];
  const reqItems = state.req.items;
  const result: MenuItem[] = [];
  for (const entry of selected) {
    const item = typeof entry === "number" ? reqItems.at(entry) : entry;
    if (item != null) result.push(item);
  }
  return result;
});

// The viewport clamp is in css, so it only needs the click position
const menuPosition = computed(() => ({
  top: `${posY.value}px`,
  left: `${posX.value}px`,
  "--menu-x": `${posX.value}px`,
  "--menu-y": `${posY.value}px`,
}));

const hasExplicitItems = computed(() => Array.isArray(props.items) && props.items.length > 0);

const selectedCount = computed(() =>
  hasExplicitItems.value ? (props.items?.length ?? 0) : state.selected.length,
);

const firstSelected = computed<MenuItem | null>(() => {
  if (hasExplicitItems.value) return props.items?.[0] ?? null;
  const entry = state.selected[0];
  return (typeof entry === "number" ? state.req.items.at(entry) : entry) ?? null;
});

/** Distinct sources of the selection, without building the item list. */
const selectionSources = computed<string[]>(() => {
  const fallback = state.sources?.current || "";
  const seen = new Set<string>();
  if (hasExplicitItems.value) {
    for (const item of props.items ?? []) seen.add(item.source || fallback);
  } else if (state.selected.length > 0) {
    const reqItems = state.req.items;
    for (const entry of state.selected) {
      const item = typeof entry === "number" ? reqItems.at(entry) : entry;
      if (item != null) seen.add(item.source || fallback);
    }
  }
  return [...seen];
});

const permissions = computed(() => {
  const global = getters.globalPermissions();
  const sources = selectionSources.value;

  if (sources.length > 0) {
    const merged = { view: true, download: true, modify: true, create: false, delete: true };
    for (const source of sources) {
      const perms = getters.sourcePermissions(source);
      merged.view = merged.view && perms.view;
      merged.download = merged.download && perms.download;
      merged.modify = merged.modify && perms.modify;
      merged.create = merged.create || perms.create;
      merged.delete = merged.delete && perms.delete;
    }
    return { ...global, ...merged };
  }

  const source = state.req?.source || state.sources?.current || "";
  return { ...global, ...getters.sourcePermissions(source) };
});

const isLoggedIn = computed(() => getters.isLoggedIn());
const isSearchActive = computed(() => state.isSearchActive);
const isAdvancedSearch = computed(() => getters.currentTool()?.component === "AdvancedSearch");

const showContext = computed(() => getters.currentPromptName() === "ContextMenu");
const showOverflow = computed(() => getters.currentPromptName() === "OverflowMenu");
const isOpen = computed(() => showContext.value || showOverflow.value);

const centered = computed(
  () => props.showCentered || getters.isMobile() || !posX.value || !posY.value,
);

const menuKey = computed(() => {
  if (showNewFileTemplate.value) return "template-mode";
  return showCreate.value ? "create-mode" : "normal-mode";
});

/** Whether the (+) panel can be opened (create file ops and/or admin-only rows like access rules). */
const canOpenCreatePanel = computed(() => {
  if (getters.isShare()) {
    return !!getters.sourcePermissions().create;
  }
  return getters.isAdmin() || !!getters.sourcePermissions().create;
});

/** New folder / new file / upload requires permissions.create only (not admin alone) */
const showCreateFileActions = computed(() => {
  if (props.showLimitedOptions || isAdvancedSearch.value) return false;
  if (!permissions.value.create) return false;
  return showCreate.value && !isSearchActive.value;
});

const showInfo = computed(() => {
  if (props.showLimitedOptions) return selectedCount.value === 1;
  return !showCreate.value && selectedCount.value === 1;
});

const showDownload = computed(() => {
  if (props.showLimitedOptions) return false;
  return !showCreate.value && !!permissions.value.download && selectedCount.value > 0;
});

const showUnarchive = computed(() => {
  if (props.showLimitedOptions || getters.isShare()) return false;
  if (!permissions.value.create || showCreate.value) return false;
  if (selectedCount.value !== 1) return false;
  const item = firstSelected.value;
  return !!item && isArchivePath(item.path || item.from || item.name);
});

const showArchive = computed(() => {
  if (props.showLimitedOptions || getters.isShare()) return false;
  if (!permissions.value.create) return false;
  return (
    showCreate.value &&
    selectedCount.value > 0 &&
    !showUnarchive.value &&
    !isSearchActive.value
  );
});

const showShareAction = computed(() => {
  if (props.showLimitedOptions) return false;
  if (isAdvancedSearch.value && selectedCount.value === 0) return false;
  return selectedCount.value <= 1 && !getters.isShare() && !!permissions.value.share;
});

const showSendToApp = computed(() => {
  if (!canNativeShare()) return false;
  if (props.showLimitedOptions || showCreate.value) return false;
  if (!permissions.value.download || selectedCount.value !== 1) return false;
  const item = firstSelected.value;
  return !!item && !isFolderItem(item);
});

const showRename = computed(() => {
  if (props.showLimitedOptions) return false;
  return (
    !showCreate.value &&
    selectedCount.value === 1 &&
    !!permissions.value.modify &&
    !isSearchActive.value
  );
});

const showPinAction = computed(() => {
  if (props.showLimitedOptions) return false;
  if (!getters.isLoggedIn()) return false;
  if (getters.isShare() && !state.shareInfo?.canEditShare) return false;
  if (showCreate.value || isSearchActive.value) return false;
  if (selectedCount.value !== 1) return false;
  return getters.currentView() === "listingView";
});

const showCopy = computed(() => {
  if (props.showLimitedOptions) return false;
  if (showCreate.value || selectedCount.value === 0) return false;
  // A share refuses every change, copy included, unless it allows modify.
  if (getters.isShare()) return !!permissions.value.modify;
  // Copying only reads the source, so it stays available in read-only sources.
  return (
    !!permissions.value.download &&
    (state.user?.scopes ?? []).some((scope) => getters.canCreateInSource(scope.name))
  );
});

const showCopyPath = computed(() => {
  if (props.showLimitedOptions) return false;
  return !showCreate.value && selectedCount.value === 1 && !!state.user?.showCopyPath;
});

const showOpenParentFolder = computed(
  () =>
    !showCreate.value &&
    selectedCount.value === 1 &&
    (isSearchActive.value || props.showLimitedOptions || isAdvancedSearch.value),
);

const showGoToItem = computed(() => props.showLimitedOptions && selectedCount.value === 1);

const showMove = computed(() => {
  if (props.showLimitedOptions) return false;
  return !showCreate.value && selectedCount.value > 0 && !!permissions.value.modify;
});

const showSelectAll = computed(() => {
  if (props.showLimitedOptions) return false;
  return !showCreate.value && !isSearchActive.value && (state.req?.items?.length ?? 0) > 0;
});

const showDelete = computed(() => {
  if (props.showLimitedOptions || selectedCount.value === 0) return false;
  return (
    getters.currentView() !== "settings" &&
    !isSearchActive.value &&
    !!permissions.value.delete
  );
});

const showSelectMultiple = computed(() => {
  if (props.showLimitedOptions) return false;
  if (state.multiple || isSearchActive.value) return false;
  if (isAdvancedSearch.value) return true;
  if (state.user?.showSelectMultiple) return true;
  return getters.isMobile();
});

const showAccess = computed(() => {
  if (props.showLimitedOptions || getters.isShare()) return false;
  if (selectedCount.value > 1) return false;
  return getters.isAdmin() && showCreate.value;
});

const showQuotaAction = computed(() => {
  if (props.showLimitedOptions || getters.isShare()) return false;
  if (!getters.isAdmin() || !showCreate.value || isSearchActive.value) return false;
  if (selectedCount.value > 1) return false;
  if (selectedCount.value === 1) return isFolderItem(firstSelected.value);
  return !isAdvancedSearch.value;
});

/** + control in listing / share directory context; disabled when the panel cannot be opened. */
const showCreateToggle = computed(() => {
  if (props.showLimitedOptions || props.createOnly) return false;
  return !isSearchActive.value;
});

const createToggleDisabled = computed(() => !canOpenCreatePanel.value);

const showDivider = computed(() => {
  if (props.showLimitedOptions || props.createOnly) return false;
  return showCreateToggle.value || selectedCount.value > 0;
});

const hasDownload = computed(() => selectedCount.value > 0 && !!permissions.value.download);
const isPreview = computed(() => getters.isPreviewView());

const showWatch = computed(() => {
  if (isMediaRequest(state.req)) return false;
  return hasDownload.value && !state.req.isDir && !getters.isShare();
});

const showGoToRaw = computed(() => {
  if (!permissions.value.download || isMediaRequest(state.req)) return false;
  const view = getters.currentView();
  return view === "preview" || view === "markdownViewer" || view === "editor";
});

const showUnarchiveInOverflow = computed(() => {
  if (!permissions.value.create || getters.isShare()) return false;
  const req = state.req;
  return !!req && !req.isDir && isArchivePath(req.path || req.name);
});

const showSendToAppInPreview = computed(() => {
  if (!canNativeShare()) return false;
  if (!hasDownload.value || state.req?.isDir) return false;
  return isPreview.value;
});

const showEditButton = computed(() => {
  if (getters.isSplitViewActive() || getters.currentView() === "editor") return false;
  const allowEdit =
    permissions.value.modify || (getters.isShare() && state.shareInfo?.allowModify);
  return isRichTextPreviewMimeType(state.req.type) && !!allowEdit;
});

const showPreviewButton = computed(() => {
  if (getters.isSplitViewActive()) return false;
  if (getters.currentView() !== "editor") return false;
  return isRichTextPreviewMimeType(state.req.type);
});

const showParentFolderInPreview = computed(() => isPreview.value && state.req.path !== "/");

// The header overflow button only exists outside the listing and advanced search views.
const hasOverflowItems = computed(
  () =>
    getters.currentView() !== "listingView" &&
    !isAdvancedSearch.value &&
    (showDelete.value ||
      showGoToRaw.value ||
      hasDownload.value ||
      showUnarchiveInOverflow.value ||
      showEditButton.value ||
      showPreviewButton.value),
);

const newFileTemplates = computed(() =>
  Array.isArray(state.user?.newFileTemplate) ? state.user.newFileTemplate : [],
);

const newFileTemplateItems = computed(() =>
  newFileTemplates.value.map((template) => {
    const typeInfo = getTypeInfoFromExt(template);
    return {
      name: template,
      icon: typeInfo.materialSymbol as string,
      iconClass: typeInfo.classes as string,
    };
  }),
);

/** Raw objects in a raw array keep Vue from building deep proxies for large selections. */
function itemsForPrompt(): MenuItem[] {
  return markRaw(providedItems.value.map((item) => toRaw(item)));
}

function showInfoPrompt() {
  mutations.closeHovers();
  mutations.showPrompt({ name: "info", props: { item: firstSelected.value } });
}

function navToItem() {
  const item = firstSelected.value;
  if (!item) return;
  goToItem(item.source, item.path, {}, true, getters.isShare());
  mutations.closeHovers();
}

function showAccessPrompt() {
  mutations.closeHovers();
  const item = firstSelected.value;
  mutations.showPrompt({
    name: "access",
    props: {
      sourceName: item?.source || state.req.source,
      path: item?.path || state.req.path,
    },
  });
}

function showQuotaPrompt() {
  mutations.closeHovers();
  const target = firstSelected.value || state.req;
  mutations.showPrompt({
    name: "Quota",
    props: {
      item: target,
      source: target?.source || state.req.source,
      path: target?.path || state.req.path,
    },
  });
}

function showSharePrompt() {
  mutations.closeHovers();
  mutations.showPrompt({
    name: "share",
    props: { item: selectedCount.value === 1 ? firstSelected.value : state.req },
  });
}

function showRenamePrompt(
  item = selectedCount.value === 1 ? firstSelected.value : state.req,
  parentItems: unknown = [],
) {
  mutations.closeHovers();
  mutations.showPrompt({ name: "rename", props: { item, parentItems } });
}

/** Folder the new item is created in, when the menu was opened on a directory. */
function templateBase() {
  const item = firstSelected.value;
  return item?.isDir ? { path: item.path, source: item.source } : null;
}

function showNewDirPrompt() {
  mutations.closeHovers();
  mutations.showPrompt({ name: "newDir", props: { base: templateBase() } });
}

function createFromTemplate(template: string) {
  mutations.closeHovers();
  mutations.showPrompt({
    name: "newFile",
    props: { base: templateBase(), initialName: template },
  });
}

function createEmptyFile() {
  mutations.closeHovers();
  mutations.showPrompt({ name: "newFile", props: { base: templateBase() } });
}

function showUploadPrompt() {
  mutations.closeHovers();
  const item = firstSelected.value;
  const target = item?.isDir ? item : state.req;
  mutations.showPrompt({
    name: "upload",
    props: { targetPath: target.path, targetSource: target.source },
  });
}

function toggleMultipleSelection() {
  mutations.setMultiple(true);
  mutations.closeHovers();
}

function startDownload() {
  mutations.closeTopPrompt();
  downloadFiles(providedItems.value.map((item) => toRaw(item)));
}

async function sendToApp(item: MenuItem | ReqObject | null = firstSelected.value) {
  mutations.closeTopPrompt();
  if (!item) return;
  await nativeShareFile(item);
}

function showDeletePrompt() {
  mutations.closeTopPrompt();
  mutations.showPrompt({ name: "delete", props: { items: itemsForPrompt() } });
}

function showMovePrompt() {
  mutations.closeHovers();
  mutations.showPrompt({
    name: "move",
    props: { items: itemsForPrompt(), operation: "move" },
  });
}

function showCopyPrompt() {
  mutations.closeHovers();
  mutations.showPrompt({
    name: "copy",
    props: { items: itemsForPrompt(), operation: "copy" },
  });
}

async function togglePin() {
  const item = firstSelected.value;
  if (!item?.name || !getters.isLoggedIn()) return;
  if (getters.isShare() && !state.shareInfo?.canEditShare) return;
  mutations.closeHovers();
  const directoryPath = getParentDir(item.path);
  const action = item.pinned ? "remove" : "add";
  try {
    if (getters.isShare()) {
      await shareApi.patchPinnedItem({
        hash: state.shareInfo.hash,
        path: directoryPath,
        name: item.name,
        action,
      });
    } else {
      const source = item.source || state.req?.source || state.sources.current;
      if (!source) return;
      await usersApi.patchPinnedItem({ source, path: directoryPath, name: item.name, action });
    }
    mutations.setReload(true);
  } catch (e) {
    notify.showError(e);
  }
}

async function copyPathToClipboard() {
  await copyToClipboard(firstSelected.value?.path || "");
  mutations.closeHovers();
}

function showArchivePrompt() {
  mutations.closeTopPrompt();
  const items = providedItems.value.map((item) => ({
    path: item.path,
    name: item.name,
    source: item.source || state.req.source,
  }));
  if (items.length === 0) return;
  mutations.showPrompt({
    name: "archive",
    props: {
      items,
      source: state.req.source,
      currentPath: state.req.path || "/",
    },
  });
}

function showUnarchivePrompt(item: MenuItem | ReqObject | null = firstSelected.value) {
  mutations.closeTopPrompt();
  if (!item) return;
  mutations.showPrompt({
    name: "unarchive",
    props: {
      item: {
        path: item.path || (item as MenuItem).from,
        source: item.source || state.req.source,
        name: item.name,
      },
    },
  });
}

function openParentFolder() {
  const item = firstSelected.value;
  if (!item) return;
  const parentPath = removeLastDir(item.path) || "/";
  goToItem(item.source, parentPath, {}, props.showLimitedOptions);
  mutations.closeHovers();
}

function goToRaw() {
  const source = state.req?.source || "";
  const path = state.req?.path || "";
  const openUrl = getters.isShare()
    ? resourcesApi.getOpenFileURL(source, path, {
        path: state.shareInfo.subPath,
        hash: state.shareInfo.hash,
      })
    : resourcesApi.getOpenFileURL(source, path);
  window.open(openUrl, "_blank");
  mutations.closeHovers();
}

function watchFile() {
  mutations.closeHovers();
  void router.push({
    path: "/tools/fileWatcher",
    query: {
      path: state.req?.path || "/",
      source: state.req?.source || state.sources.current || "",
    },
  });
}

function goToEdit() {
  mutations.closeHovers();
  void router.replace({ hash: "#edit" });
}

function goToPreview() {
  mutations.closeHovers();
  void router.replace({ hash: "#preview" });
}

function addAction(
  list: MenuAction[],
  visible: boolean,
  id: string,
  icon: string,
  label: string,
  run: () => void | Promise<void>,
) {
  if (visible) list.push({ id, icon, label, run });
}

const mainActions = computed<MenuAction[]>(() => {
  const list: MenuAction[] = [];

  addAction(list, showCreateFileActions.value, "new-folder", "create_new_folder", t("files.newFolder"), showNewDirPrompt);
  addAction(list, showCreateFileActions.value, "new-file", "note_add", t("files.newFile"), () => {
    showNewFileTemplate.value = true;
  });
  addAction(list, showCreateFileActions.value, "upload", "file_upload", t("general.upload"), showUploadPrompt);
  addAction(list, showQuotaAction.value, "quota", "storage", t("quotas.title"), showQuotaPrompt);
  addAction(list, showAccess.value, "access", "lock", t("access.rules"), showAccessPrompt);
  addAction(list, showArchive.value, "archive", "archive", t("prompts.archive"), showArchivePrompt);
  addAction(list, showInfo.value, "info", "info", t("general.info"), showInfoPrompt);
  addAction(list, showDownload.value, "download", "file_download", t("general.download"), startDownload);
  addAction(list, showSendToApp.value, "send-to-app", "ios_share", t("buttons.sendToApp"), sendToApp);
  addAction(list, showUnarchive.value, "unarchive", "unarchive", t("prompts.unarchive"), showUnarchivePrompt);
  addAction(list, showShareAction.value, "share", "share", t("general.share"), showSharePrompt);
  addAction(list,
    showPinAction.value,
    "pin",
    "push_pin",
    firstSelected.value?.pinned ? t("buttons.unpinItem") : t("buttons.pinItem"),
    togglePin,
  );
  addAction(list, showRename.value, "rename", "edit", t("general.rename"), showRenamePrompt);
  addAction(list, showCopy.value, "copy", "file_copy", t("buttons.copyFile"), showCopyPrompt);
  addAction(list, showCopyPath.value, "copy-path", "copy_all", t("buttons.copyPath"), copyPathToClipboard);
  addAction(list, showOpenParentFolder.value, "open-parent", "folder", t("buttons.openParentFolder"), openParentFolder);
  addAction(list, showGoToItem.value, "go-to-item", "folder", t("buttons.goToItem"), navToItem);
  addAction(list, showMove.value, "move", "forward", t("buttons.moveFile"), showMovePrompt);
  addAction(list, showSelectAll.value, "select-all", "select_all", t("buttons.selectAll"), () => mutations.selectAllItems());
  addAction(list, showDelete.value, "delete", "delete", t("general.delete"), showDeletePrompt);
  addAction(list,
    showSelectMultiple.value,
    "select-multiple",
    "check_circle",
    t("buttons.selectMultiple"),
    toggleMultipleSelection,
  );
  return list;
});

const overflowActions = computed<MenuAction[]>(() => {
  const list: MenuAction[] = [];

  addAction(list, true, "info", "info", t("general.info"), showInfoPrompt);
  addAction(list, showGoToRaw.value, "open-file", "open_in_new", t("general.openFile"), goToRaw);
  addAction(list, showParentFolderInPreview.value, "open-parent", "folder", t("buttons.openParentFolder"), openParentFolder);
  addAction(list,
    isPreview.value && !!permissions.value.modify,
    "rename",
    "edit",
    t("general.rename"),
    () => showRenamePrompt(state.req, state.navigation.listing || []),
  );
  addAction(list, showWatch.value, "watch", "visibility", t("buttons.watchFile"), watchFile);
  addAction(list, hasDownload.value, "download", "file_download", t("general.download"), startDownload);
  addAction(list, showSendToAppInPreview.value, "send-to-app", "ios_share", t("buttons.sendToApp"), () => sendToApp(state.req));
  addAction(list,
    showUnarchiveInOverflow.value,
    "unarchive",
    "folder_open",
    t("prompts.unarchive"),
    () => showUnarchivePrompt(state.req),
  );
  addAction(list, showEditButton.value, "edit", "edit", t("general.edit"), goToEdit);
  addAction(list, showPreviewButton.value, "preview", "visibility", t("general.preview"), goToPreview);
  addAction(list, showDelete.value, "delete", "delete", t("general.delete"), showDeletePrompt);
  return list;
});

function onCreateToggleClick() {
  if (createToggleDisabled.value) return;
  showCreate.value = canOpenCreatePanel.value ? !showCreate.value : false;
}

function onCreateToggleMouseEnter(event: MouseEvent) {
  if (!createToggleDisabled.value) return;
  showHoverTooltip(t("messages.noCreateOptions"), event);
}

function initializeCreateState() {
  showNewFileTemplate.value = false;
  showCreate.value = props.createOnly && !isAdvancedSearch.value;
}

function setPositions() {
  const contextProps = getters.currentPrompt()?.props;
  posX.value = Number(contextProps?.posX) || 0;
  posY.value = Number(contextProps?.posY) || 0;
}

watch(
  showContext,
  (open) => {
    if (open) {
      setPositions();
      initializeCreateState();
    } else {
      showNewFileTemplate.value = false;
    }
  },
  { immediate: true },
);

watch(hasOverflowItems, (hasItems) => mutations.setContextMenuHasItems(hasItems), {
  immediate: true,
});

function onEscapeKey(event: KeyboardEvent) {
  if (event.key !== "Escape" || !isOpen.value) return;
  event.preventDefault();
  event.stopPropagation();
  mutations.closeTopPrompt();
}

onMounted(() => document.addEventListener("keydown", onEscapeKey, true));
onBeforeUnmount(() => document.removeEventListener("keydown", onEscapeKey, true));
</script>

<style scoped>
#context-menu {
  position: absolute;
  z-index: 1000;
  background-color: var(--background);
  max-width: 20em;
  min-width: 13em;
  min-height: 4em;
  max-height: calc(100dvh - 16px);
  height: auto;
  display: flex;
  flex-direction: column;
  border-radius: var(--borderRadius);
  padding: 0.5em;
  overflow-y: auto;
  overscroll-behavior: contain;
  transform-origin: top left;
}

#context-menu:not(.centered, .overflow-menu) {
  translate:
    max(calc(8px - var(--menu-x, 0px)), min(0px, calc(100dvw - 8px - var(--menu-x, 0px) - 100%)))
    max(calc(8px - var(--menu-y, 0px)), min(0px, calc(100dvh - 8px - var(--menu-y, 0px) - 100%)));
}

#context-menu > :first-child {
  margin-top: auto;
}

#context-menu > :last-child {
  margin-bottom: auto;
}

#context-menu.centered {
  top: 50% !important;
  left: 50% !important;
  translate: -50% -50%;
  transform-origin: center;
}

#context-menu.overflow-menu {
  top: 3em;
  right: 1em;
  max-height: calc(100dvh - 3em - 8px);
  transform-origin: top right;
}

.selected-count-header {
  border-radius: var(--borderRadius);
  cursor: unset;
}

/* Circle for up to 3 digits; 1000+ keeps pill shape from .button */
.selected-count-header--circle {
  box-sizing: border-box;
  flex-shrink: 0;
  width: 2em;
  height: 2em;
  min-width: 2em;
  min-height: 2em;
  padding: 0;
  border-radius: 50%;
}

.context-menu-header > .action i {
  padding: 0.25em;
}

#context-menu .context-menu-create-toggle {
  padding: 0.2em;
}

.context-menu-create-toggle--disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.context-menu-empty-message {
  padding: 0.5em 1em;
  margin: 0;
  font-size: 0.85em;
  color: var(--textSecondary);
  text-align: center;
}

#context-menu .action {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-shrink: 0;
}

.context-menu-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-left: 0.5em;
  padding-right: 0.5em;
}

#context-menu > .context-menu-header,
#context-menu > hr.divider {
  flex-shrink: 0;
}

#context-menu > div,
#context-menu > button {
  width: 100%;
}

#context-menu .action > span {
  padding-right: 0.75em;
}

#context-menu .action:hover :deep(.material-symbols-outlined) {
  font-variation-settings: 'FILL' 1;
}
</style>
