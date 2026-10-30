import { onBeforeUnmount, onMounted, type Ref } from "vue";
import downloadFiles from "@/utils/download.js";
import { resourcesApi } from "@/api/index.js";
import { router } from "@/router/index.ts";
import { state, mutations, getters } from "@/store/index.ts";
import type { FileListItem, fileListItemIndex } from "@/store/types.ts";
import { url } from "@/utils/index.ts";
import {
  isTypeAheadSessionActive,
  processTypeAheadKey,
  resetTypeAheadSession,
} from "@/utils/listingTypeAhead.js";
import { useListingClipboard } from "@/composables/useListingClipboard.ts";

interface ListingKeyboardOptions {
  container: Ref<HTMLElement | null>;
  items: () => fileListItemIndex[];
  isActive: () => boolean;
  canModify: () => boolean;
  // Views with text fields on the page must not hijack keys typed in them
  ignoreEditableTargets?: boolean;
  // Backspace and Alt+ArrowUp go to the parent folder; search results has none
  parentNavigation?: boolean;
  // Return true when the key was consumed
  onEscape?: () => boolean;
  // For views that have to remember where to come back to when an item is opened
  openItem?: (item: FileListItem) => void;
}

function isEditableTarget(target: EventTarget | null) {
  const el = target as HTMLElement | null;
  const tag = el?.tagName ? el.tagName.toLowerCase() : "";
  return tag === "input" || tag === "textarea" || tag === "select" || !!el?.isContentEditable;
}

export function useListingKeyboard(options: ListingKeyboardOptions) {
  const { copyCut } = useListingClipboard();

  function showDeletePrompt() {
    const promptItems = [];
    for (const index of state.selected as number[]) {
      const item = state.req.items.at(index);
      if (!item) continue;
      const previewUrl = item.hasPreview
        ? resourcesApi.getPreviewURL(item.source || state.req.source, item.path, item.modified)
        : null;
      promptItems.push({
        source: item.source || state.req.source,
        path: item.path,
        type: item.type,
        size: item.size,
        modified: item.modified,
        created: item.created,
        hasPreview: item.hasPreview,
        previewUrl: previewUrl,
      });
    }
    mutations.showPrompt({
      name: "delete",
      props: { items: promptItems },
    });
  }

  // select the first item if nothing is selected
  function selectFirstItem() {
    mutations.resetSelected();
    const first = options.items()[0];
    if (first) {
      mutations.addSelected(first.index);
    }
  }

  // find the closest item in the given direction (up or down) from the current one
  function findClosestItem(selectedItem: Element, direction: "up" | "down") {
    const listItems = options.container.value?.querySelectorAll(".listing-item") ?? [];
    const selectedIndex = Array.prototype.indexOf.call(listItems, selectedItem);
    if (selectedIndex === -1) return null;

    const selectedBounds = selectedItem.getBoundingClientRect();
    const selectedMidX = (selectedBounds.left + selectedBounds.right) / 2;
    const down = direction === "down";

    let closestItem: Element | null = null;
    let closestDistance = Infinity;

    const step = down ? 1 : -1;
    for (let i = selectedIndex + step; i >= 0 && i < listItems.length; i += step) {
      const item = listItems[i] as Element;
      const itemBounds = item.getBoundingClientRect();
      const verticalGap = down
        ? itemBounds.top - selectedBounds.bottom
        : selectedBounds.top - itemBounds.bottom;

      if (verticalGap <= 0) continue; // same row as the selected item
      if (verticalGap >= closestDistance) break;

      const itemMidX = (itemBounds.left + itemBounds.right) / 2;
      const distance = Math.hypot(Math.abs(itemMidX - selectedMidX), verticalGap);
      if (distance < closestDistance) {
        closestDistance = distance;
        closestItem = item;
      }
    }
    return closestItem;
  }

  function moveSelectionBy(step: number) {
    const all = options.items();
    const selectedIndex = state.selected.length > 0 ? state.selected[0] : null;
    if (selectedIndex === null) {
      selectFirstItem();
      return false;
    }
    const currentItemIndex = all.findIndex((item) => item.index === selectedIndex);
    if (currentItemIndex === -1) {
      selectFirstItem();
      return false;
    }
    const nextItemIndex = currentItemIndex + step;
    const targetItem = all.at(nextItemIndex);
    if (nextItemIndex >= 0 && targetItem) {
      mutations.resetSelected();
      mutations.addSelected(targetItem.index);
      scrollSelectedIntoView();
      return true;
    }
    return false;
  }

  // handle selection based on arrow keys
  function navigateKeyboardArrows(arrowKey: string) {
    const isCardView = getters.isCardView(); // gallery, normal, icons
    const selectedIndex = state.selected.length > 0 ? state.selected[0] : null;

    if (selectedIndex === null) {
      selectFirstItem();
      return;
    }

    // Left/Right arrows will always use the visual order that we see
    if (arrowKey === "ArrowLeft" || arrowKey === "ArrowRight") {
      const step = arrowKey === "ArrowLeft" ? -1 : 1;
      moveSelectionBy(step);
      return;
    }

    // On list/compact views we use a simple linear navigation since all the items are aligned
    if (!isCardView) {
      const step = arrowKey === "ArrowUp" ? -1 : 1;
      moveSelectionBy(step);
      return;
    }

    // but for gallery, normal, icons views, we need to find the closest item
    // cuz the rows aren't always consistent (some have 1 item, others 5, etc)
    const selectedItem = options.container.value?.querySelector(`.listing-item[data-index="${selectedIndex}"]`);
    if (!selectedItem) return;

    let nextItem: Element | null = null;

    switch (arrowKey) {
      case "ArrowDown":
        nextItem = findClosestItem(selectedItem, "down");
        break;
      case "ArrowUp":
        nextItem = findClosestItem(selectedItem, "up");
        break;
    }

    const itemIndex = parseInt((nextItem as HTMLElement | null)?.dataset.index as string, 10);
    if (!Number.isNaN(itemIndex)) {
      mutations.resetSelected();
      mutations.addSelected(itemIndex);
      scrollSelectedIntoView();
    }
  }

  function scrollSelectedIntoView() {
    setTimeout(() => {
      const element = document.querySelector(
        '.listing-item[aria-selected="true"]'
      );
      if (element) {
        element.scrollIntoView({
          behavior: "smooth",
          block: "nearest",
          inline: "nearest",
        });
      }
    }, 50);
  }

  function keyEvent(event: KeyboardEvent) {
    const { key, ctrlKey, metaKey, altKey, which } = event;
    const isArrowKey = key === "ArrowUp" ||
                       key === "ArrowDown" ||
                       key === "ArrowLeft" ||
                       key === "ArrowRight";
    if (state.isSearchActive || !options.isActive() ||
      getters.currentPromptName() || (event.repeat && (!isArrowKey || altKey))) return;

    if (options.ignoreEditableTargets && isEditableTarget(event.target)) return;

    if (
      (key === " " || key === "/") &&
      isTypeAheadSessionActive() &&
      !state.isSearchActive &&
      !getters.currentPromptName()
    ) {
      event.preventDefault();
      return;
    }

    const isAlphanumeric = /^[a-z0-9]$/i.test(key);
    const modifierKeys = ctrlKey || metaKey;
    if (isAlphanumeric && !modifierKeys && state.selected.length <= 1) {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName ? target.tagName.toLowerCase() : "";
      if (
        tag !== "input" &&
        tag !== "textarea" &&
        tag !== "select" &&
        !target?.isContentEditable
      ) {
        event.preventDefault();
        alphanumericKeyPress(key);
        return;
      }
    }

    if (modifierKeys) {
      const charKey = String.fromCharCode(which).toLowerCase();

      switch (charKey) {
        case "c":
        case "x":
          copyCut(event, charKey);
          return;
        case "a":
          event.preventDefault();
          mutations.selectAllItems({ multiple: false });
          return;
        case "d":
          event.preventDefault();
          downloadFiles(state.selected);
          return;
      }
    }

    let shortcut = key;
    if (altKey) shortcut = `Alt+${key}`;

    switch (shortcut) {
      case "Alt+ArrowUp":
      case "Backspace": {
        if (options.parentNavigation === false) break;
        event.preventDefault();
        // get current path and its parent
        const currentPath = state.req.path || "/";
        const parentPath = url.removeLastDir(currentPath);
        if (parentPath === currentPath) {
          return;
        }
        const source = getters.isShare() ? state.shareInfo.hash : state.req.source;
        const newPath = url.buildItemUrl(source, parentPath);
        void router.push({ path: newPath });
        break;
      }

      case "Alt+ArrowDown":
      case "Enter": {
        event.preventDefault();
        if (state.selected.length === 1) {
          const selected = getters.getFirstSelected();
          if (!selected) return;
          if (options.openItem) {
            options.openItem(selected);
            break;
          }
          const selectedUrl = url.buildItemUrl(selected.source, selected.path);
          if (selectedUrl === state.route.path) return;
          void router.push({ path: selectedUrl });
        }
        break;
      }

      case "Escape":
        if (options.onEscape?.()) {
          event.preventDefault();
          return;
        }
        mutations.resetSelected();
        break;

      case "Delete":
        if (!options.canModify() || state.selected.length === 0) return;
        showDeletePrompt();
        break;

      case "ArrowUp":
      case "ArrowDown":
      case "ArrowLeft":
      case "ArrowRight":
        event.preventDefault();
        navigateKeyboardArrows(key);
        break;
    }
  }

  function alphanumericKeyPress(key: string) {
    const selectedIndex = state.selected.length === 1 ? state.selected[0] : null;
    const { matches, nextPos } = processTypeAheadKey(key, options.items(), selectedIndex as number | null);
    if (matches.length === 0) {
      return;
    }

    const target = matches.at(nextPos);
    if (!target) return;
    mutations.resetSelected();
    mutations.addSelected(target.index);
    scrollSelectedIntoView();
  }

  onMounted(() => {
    window.addEventListener("keydown", keyEvent);
  });

  onBeforeUnmount(() => {
    resetTypeAheadSession();
    window.removeEventListener("keydown", keyEvent);
  });
}
