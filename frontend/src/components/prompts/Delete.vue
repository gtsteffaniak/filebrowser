<template>
    <div class="card-content">
      <!-- Loading spinner overlay -->
      <div v-show="deleting" class="loading-content">
        <LoadingSpinner size="small" />
        <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
      </div>
      <div v-show="!deleting">
        <p class="card-message" v-if="itemsToDelete.length === 1">
          {{ $t("prompts.deleteMessageSingle") }}
        </p>
        <p class="card-message" v-else>
          {{ $t("prompts.deleteMessageMultiple", { count: itemsToDelete.length }) }}
        </p>
        <div ref="list" class="delete-items-list" @contextmenu.capture.prevent.stop>
          <div data-virtual-group="items" :style="virtual.group('items').style">
            <div
              v-for="entry in virtual.group('items').items"
              :key="`${entry.item.source}::${entry.item.path}`"
              class="delete-item-wrapper"
              :class="{ 'has-error': getItemError(entry.item) }"
            >
              <ListingItem
                :name="getItemName(entry.item.path)"
                :isDir="entry.item.type === 'directory'"
                :source="entry.item.source"
                :type="entry.item.type"
                :size="entry.item.size"
                :modified="entry.item.modified"
                :index="entry.index"
                :path="entry.item.path"
                :hasPreview="entry.item.hasPreview"
                :displayFullPath="true"
                :updateGlobalState="false"
                :isSelectedProp="false"
                :clickable="false"
                @click="preventInteraction"
                @select="preventInteraction"
                class="delete-listing-item"
              />
              <div v-if="getItemError(entry.item)" class="error-banner">
                {{ getItemError(entry.item) }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="card-actions">
      <button
        type="button"
        @click="closeTopPrompt"
        class="button button--flat button--grey"
        :disabled="deleting"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        @click="submit"
        class="button button--flat button--red"
        :disabled="deleting"
        aria-label="Confirm-Delete"
        :title="$t('general.delete')">
        {{ $t("general.delete") }}
      </button>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { resourcesApi } from "@/api";
import buttons from "@/utils/buttons";
import { state, getters, mutations } from "@/store";
import type { FileListItem } from "@/store/types";
import { notify } from "@/notify";
import ListingItem from "@/components/files/ListingItem.vue";
import { eventBus } from "@/store/eventBus";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { useListingVirtualization } from "@/composables/useListingVirtualization.ts";
import url from "@/utils/url";
import { removeFromFolderLists } from "@/utils/cache/listingCache.ts";

export interface DeleteItem {
  source?: string;
  path: string;
  type?: string;
  size?: number;
  modified?: string;
  created?: string;
  hasPreview?: boolean;
}

interface FailedItem {
  source?: string;
  path: string;
  message?: string;
}

const props = withDefaults(
  defineProps<{
    promptId?: string | number | null;
    items?: DeleteItem[] | null;
  }>(),
  { promptId: null, items: null },
);

defineOptions({ name: "delete" });

const { t } = useI18n();

const list = ref<HTMLElement | null>(null);
const failedItems = ref<FailedItem[]>([]);
const deleting = ref(false);

const failedMessages = computed(
  () => new Map(failedItems.value.map((item) => [`${item.source}::${item.path}`, item.message || "Unknown error"])),
);

const itemsToDelete = computed<DeleteItem[]>(() => {
  // If props.items is provided, use that
  if (props.items?.length) {
    return props.items;
  }
  // Otherwise, compute from state (backward compatibility)
  let selectedItems: FileListItem[];
  if (state.isSearchActive || getters.currentView() === "preview") {
    const selected = state.selected[0] as number | FileListItem;
    selectedItems = [(typeof selected === "number" ? state.req.items?.at(selected) : selected) as FileListItem];
  } else if (!getters.isListing()) {
    selectedItems = [state.req.items.at(state.selected[0] as number) as FileListItem];
  } else {
    selectedItems = state.selected.map((index) => state.req.items.at(index as number) as FileListItem);
  }
  return selectedItems.map((item) => ({
    source: item.source || state.req.source,
    path: item.path,
    type: item.type,
    size: item.size,
    modified: item.modified,
    created: item.created,
    hasPreview: item.hasPreview,
  }));
});

const virtual = useListingVirtualization({
  container: list,
  groups: () => [
    {
      key: "items",
      count: itemsToDelete.value.length,
      at: (index) => ({ item: itemsToDelete.value[index] as DeleteItem, index }),
    },
  ],
  columnsHint: () => 1,
  scroller: (el) => el,
  itemClass: "delete-item-wrapper",
  minItems: 10,
});

watch(failedItems, () => void virtual.refresh());

onMounted(() => {
  if (itemsToDelete.value.length === 1 && state.user.deleteWithoutConfirming) {
    void submit();
  }
});

function preventInteraction(event?: Event) {
  if (event && typeof event.preventDefault === 'function') {
    event.preventDefault();
  }
  if (event && typeof event.stopPropagation === 'function') {
    event.stopPropagation();
  }
  return false;
}

function closeTopPrompt() {
  mutations.closeTopPrompt();
}

function getItemName(path: string) {
  const parts = path.split("/").filter(p => p);
  return parts[parts.length - 1] || path;
}

function getItemError(item: DeleteItem) {
  return failedMessages.value.get(`${item.source}::${item.path}`) ?? null;
}

function parseBulkDeleteFailures(error: { failed?: FailedItem[]; message?: string }): FailedItem[] | null {
  if (error?.failed?.length) {
    return error.failed;
  }
  if (!error?.message) {
    return null;
  }
  try {
    const parsed = JSON.parse(error.message);
    if (Array.isArray(parsed?.failed) && parsed.failed.length > 0) {
      return parsed.failed;
    }
  } catch (_e) {
    // Not JSON — fall through
  }
  return null;
}

async function submit() {
  if (deleting.value) {
    return;
  }

  deleting.value = true;
  failedItems.value = [];
  buttons.loading("delete");
  // The selection and request change while deleting, so work on what was selected at the start
  const toDelete = itemsToDelete.value;

  try {
    // Extract source and path from items
    const itemsForDelete = toDelete.map((item) => ({
      source: item.source,
      path: item.path,
    }));
    if (itemsForDelete.length === 0) {
      buttons.done("delete");
      deleting.value = false;
      return;
    }
    // Use bulk delete API for both regular files and shares
    const response = getters.isShare()
      ? await resourcesApi.bulkDeletePublic(itemsForDelete)
      : await resourcesApi.bulkDelete(itemsForDelete);

    const failed = Array.isArray(response?.failed) ? response.failed : [];
    const succeeded = Array.isArray(response?.succeeded) ? response.succeeded : [];
    failedItems.value = failed;

    const succeededCount = succeeded.length;
    const failedCount = failed.length;
    mutations.removeFromSearchResults(succeeded);

    if (failedCount === 0) {
      // All succeeded - close prompt and handle navigation
      buttons.success("delete");
      notify.showSuccessToast(t('prompts.deleted'));

      if (props.items && props.items.length > 0) {
        eventBus.emit("itemsDeleted", {
          succeeded,
          failed,
        });
      }
      mutations.closeTopPrompt();
      if (!props.items) {
        mutations.resetSelected();
      }
      if (getters.isPreviewView()) {
        for (const deleted of toDelete) {
          removeFromFolderLists(deleted);
        }
      }
      // If we delete a item while in a preview
      if (getters.isPreviewView() && toDelete.length === 1) {
        const deletedItem = toDelete[0] as DeleteItem;
        const currentItem = state.req;
        // Only navigate if we delete the current item, not by deleting a different one (from fileTree)
        if (currentItem && deletedItem.source === currentItem.source && deletedItem.path === currentItem.path) {
          removeFromFolderLists(deletedItem);
          mutations.setNavigationTransitioning(true);
          // Try next, then previous, then parent directory
          const next = state.navigation.nextItem as FileListItem | null;
          const prev = state.navigation.previousItem as FileListItem | null;
          if (next) {
            url.goToItem(getters.isShare() ? getters.shareHash() : next.source, next.path, undefined, false, getters.isShare());
          } else if (prev) {
            url.goToItem(getters.isShare() ? getters.shareHash() : prev.source, prev.path, undefined, false, getters.isShare());
          } else {
            // Navigate to parent directory of deleted item
            const parentPath = url.removeLastDir(deletedItem.path);
            url.goToItem(getters.isShare() ? getters.shareHash() : deletedItem.source, parentPath, {}, false, getters.isShare());
          }
        } else {
          removeFromFolderLists(deletedItem);
          mutations.setReload(true);
        }
      } else {
        // If someone deletes the current folder (from fileTree) that they're viewing, navigate to parent directory
        if (!getters.isPreviewView() && toDelete.length === 1) {
          const deletedItem = toDelete[0] as DeleteItem;
          if (deletedItem.type === 'directory' && deletedItem.path === state.req.path) {
            const parentPath = url.removeLastDir(deletedItem.path);
            url.goToItem(getters.isShare() ? getters.shareHash() : deletedItem.source, parentPath, {}, false, getters.isShare());
            deleting.value = false;
            return; // return early to avoid extra reload (and 404)
          }
        }
        mutations.setReload(true);
      }
    } else if (succeededCount > 0) {
      // Partial success
      buttons.done("delete");
      // Emit event with partial results if items were passed as props
      if (props.items && props.items.length > 0) {
        eventBus.emit("itemsDeleted", {
          succeeded,
          failed,
        });
      }
      if (failedCount > 0) {
        notify.showError(
          failed[0]?.message || t("prompts.operationFailed")
        );
      }
    } else {
      // All failed
      buttons.done("delete");
      if (failedCount > 0) {
        notify.showError(
          failed[0]?.message || t("prompts.operationFailed")
        );
      }
    }
    deleting.value = false;
  } catch (e) {
    const error = e as { failed?: FailedItem[]; message?: string };
    buttons.done("delete");
    deleting.value = false;
    console.error(e);
    const parsedFailures = parseBulkDeleteFailures(error);
    if (parsedFailures) {
      failedItems.value = parsedFailures;
      notify.showError(
        parsedFailures[0]?.message || t("prompts.operationFailed")
      );
    } else {
      failedItems.value = toDelete.map((item) => ({
        source: item.source,
        path: item.path,
        message: error.message || "Delete failed",
      }));
      notify.showError(error.message || t("prompts.operationFailed"));
    }
  }
}
</script>

<style scoped>
.delete-items-list {
  --icon-size: 2.5em;
  --icon-font-size: 1.8em;
  max-height: 400px;
  overflow-y: auto;
  margin-top: 1rem;
  padding: 0.5rem;
}

.delete-item-wrapper {
  margin-bottom: 0.5rem;
}

.delete-item-wrapper:last-child {
  margin-bottom: 0;
}

.delete-item-wrapper.has-error {
  border-left: 3px solid var(--errorColor, #f44336);
  padding-left: 0.5rem;
}

.delete-listing-item a {
  pointer-events: none;
  cursor: default !important;
  padding: 0.25em;
}

.delete-listing-item.listing-item {
  cursor: default !important;
  content-visibility: visible;
}

.error-banner {
  margin-top: 0.5rem;
  padding: 0.5rem;
  background: var(--errorBackground, rgb(244 67 54 / 10%));
  color: var(--errorColor, #f44336);
  border-radius: 4px;
  font-size: 0.875rem;
}

.loading-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding-top: 2em;
}

.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}

.card-content {
  position: relative;
}
</style>
