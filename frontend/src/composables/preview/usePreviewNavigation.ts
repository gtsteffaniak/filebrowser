import { onBeforeUnmount, onMounted, ref } from "vue";
import { resourcesApi } from "@/api/index.js";
import { ensureViewToken, requestViewIdentity } from "@/api/viewToken.js";
import { router } from "@/router/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import type { FileListItem } from "@/store/types.ts";
import { navigatePlaybackQueue } from "@/utils/playbackQueue.js";
import { goToItem, removeLastDir } from "@/utils/url.js";

interface PreviewNavigationOptions {
  onBeforeTeardown?: () => void;
}

/** Listing context, prev/next nav, close and key handling */
export function usePreviewNavigation(options: PreviewNavigationOptions = {}) {
  const listing = ref<FileListItem[] | null>(null);
  const listingKey = ref<string | null>(null);

  function listingContextKey(directoryPath: string): string {
    return getters.isShare()
      ? `share:${state.shareInfo?.hash || ""}:${directoryPath}`
      : `source:${state.req.source || ""}:${directoryPath}`;
  }

  /**
   * Prepares the listing for the current request: drops it when the directory
   * context changed, refreshes the view token, then adopts the listing that
   * came with the request, if any.
   */
  async function loadListingContext() {
    const currentDirectoryPath = removeLastDir(state.req.path) || "/";
    const currentListingKey = listingContextKey(currentDirectoryPath);
    if (listingKey.value !== currentListingKey) {
      listing.value = null;
    }

    if (state.req.type !== "directory" && !getters.fileViewingDisabled(state.req.name ?? "")) {
      const viewIdentity = requestViewIdentity(state.req);
      try {
        const viewToken = await ensureViewToken(state.req.source);
        if (viewToken && requestViewIdentity(state.req) === viewIdentity) {
          mutations.setRequestViewToken(viewToken);
        }
      } catch (err) {
        console.warn("Failed to refresh view token for preview:", err);
      }
    }

    if (!listing.value || (listing.value as unknown) === "undefined") {
      if (state.req.parentDirItems) {
        listing.value = state.req.parentDirItems;
        listingKey.value = currentListingKey;
      } else if (state.req.items) {
        listing.value = state.req.items;
        listingKey.value = currentListingKey;
      }
    }
  }

  /** Loads the folder listing if needed and sets up prev/next navigation for the current file. */
  async function updatePreview() {
    const expectedPath = state.req.path;
    let directoryPath = removeLastDir(state.req.path);

    if (!directoryPath || directoryPath === "") {
      directoryPath = "/";
    }
    const expectedListingKey = listingContextKey(directoryPath);

    if (!listing.value || (listing.value as unknown) === "undefined") {
      // Try to use pre-fetched parent directory items first
      if (state.req.parentDirItems) {
        listing.value = state.req.parentDirItems;
      } else if (directoryPath !== state.req.path) {
        try {
          let res: { items: FileListItem[] };
          if (getters.isShare()) {
            res = await resourcesApi.fetchFilesPublic(directoryPath, state.shareInfo?.hash);
          } else {
            res = await resourcesApi.fetchFiles(state.req.source, directoryPath);
          }
          if (
            state.req.path !== expectedPath ||
            listingContextKey(directoryPath) !== expectedListingKey
          ) {
            return;
          }
          listing.value = res.items;
        } catch (error) {
          if (state.req.path !== expectedPath) return;
          console.error("error Preview.vue", error);
          listing.value = [state.req as unknown as FileListItem];
        }
      } else {
        listing.value = [state.req as unknown as FileListItem];
      }
    }

    if (!listing.value) {
      listing.value = [state.req as unknown as FileListItem];
    }
    listingKey.value = listingContextKey(directoryPath);

    // Setup navigation using the new state management
    mutations.setupNavigation({
      listing: listing.value,
      currentItem: state.req as unknown as Partial<FileListItem>,
      directoryPath: directoryPath,
    });
  }

  function selectCurrentItem() {
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
      modified: state.req.modified,
      created: state.req.created,
      hasPreview: state.req.hasPreview,
    } as unknown as FileListItem);
  }

  function navigatePrevious() {
    if (getters.isPreviewPlaybackQueueNavMode()) {
      if (!getters.playbackQueueCanGoPrevious()) {
        return;
      }
      navigatePlaybackQueue(-1);
      return;
    }
    if (state.navigation.previousLink) {
      mutations.setNavigationTransitioning(true);
      router.replace({ path: state.navigation.previousLink });
    }
  }

  function navigateNext() {
    if (getters.isPreviewPlaybackQueueNavMode()) {
      if (!getters.playbackQueueCanGoNext()) {
        return;
      }
      navigatePlaybackQueue(1);
      return;
    }
    if (state.navigation.nextLink) {
      mutations.setNavigationTransitioning(true);
      router.replace({ path: state.navigation.nextLink });
    }
  }

  /** True when a remembered history item points at the file being previewed. */
  function isCurrentPreviewItem(item: { source?: string; path?: string; returnTo?: unknown }) {
    const current = state.req;
    if (!item?.path || !current?.path || item.returnTo) return false;
    return item.source === current.source && item.path === current.path;
  }

  function exitPreviewFromImageGesture() {
    mutations.closeHovers();
    const prev = state.previousHistoryItem;
    mutations.setPreviousHistoryItem(null);
    if (prev?.name && !isCurrentPreviewItem(prev)) {
      goToItem(prev.source, prev.path, prev, false, prev.isShare);
      return;
    }
    const parentPath = removeLastDir(state.route.path);
    router.push({ path: parentPath });
  }

  function close() {
    if (state.previousHistoryItem?.name) {
      exitPreviewFromImageGesture();
      return;
    }
    mutations.replaceRequest({} as typeof state.req); // Reset request data
    const uri = `${removeLastDir(state.route.path)}/`;
    router.push({ path: uri });
  }

  function showDeletePrompt() {
    const item = state.req;
    const previewUrl = item.hasPreview
      ? resourcesApi.getPreviewURL(item.source, item.path, item.modified)
      : null;
    mutations.showPrompt({
      name: "delete",
      props: {
        items: [
          {
            source: item.source,
            path: item.path,
            type: item.type,
            size: item.size,
            modified: item.modified,
            created: item.created,
            hasPreview: item.hasPreview,
            previewUrl: previewUrl,
          },
        ],
      },
    });
  }

  function keyEvent(event: KeyboardEvent) {
    if (getters.currentPromptName() || event.repeat) {
      return;
    }
    const { key, altKey } = event;

    let shortcut = key;
    if (altKey) shortcut = `Alt+${key}`;

    switch (shortcut) {
      case "Alt+ArrowUp":
        event.preventDefault();
        close();
        break;
      case "Escape":
      case "Backspace":
        close();
        break;
      case "Delete":
        showDeletePrompt();
        break;
    }
  }

  onMounted(() => {
    window.addEventListener("keydown", keyEvent);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("keydown", keyEvent);
    options.onBeforeTeardown?.();
    // Clear navigation state when leaving preview
    mutations.clearNavigation();
  });

  return {
    listing,
    listingKey,
    listingContextKey,
    loadListingContext,
    updatePreview,
    selectCurrentItem,
    navigatePrevious,
    navigateNext,
    exitPreviewFromImageGesture,
    close,
    showDeletePrompt,
  };
}
