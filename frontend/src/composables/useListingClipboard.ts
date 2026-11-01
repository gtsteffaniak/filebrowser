import { resourcesApi } from "@/api/index.js";
import { checkConflict } from "@/utils/upload.js";
import { state, mutations, getters } from "@/store/index.ts";
import { notifyMoveCopyFailure } from "@/utils/appNotifications.js";

interface ClipboardEntry {
  from: string;
  fromSource: string;
  name: string;
  isDir: boolean;
}

interface InternalClipboard {
  key?: string;
  items: ClipboardEntry[];
  path?: string;
}

// Module level, so what is copied in one view can be pasted from another
let clipboard: InternalClipboard = { items: [] };
let internalClipboardTimestamp = 0;

function hasInternalItems() {
  return clipboard.items.length > 0;
}

// Check if internal clipboard is recent (20 seconds)
function isInternalRecent() {
  return hasInternalItems() && (Date.now() - internalClipboardTimestamp) < 20000;
}

function copyCut(event: KeyboardEvent, key: string) {
  if ((event.target as HTMLElement).tagName.toLowerCase() === "input") {
    return;
  }

  const clipboardItems = (state.selected as number[])
    .map((i): ClipboardEntry | null => {
      const item = state.req.items.at(i);
      if (!item) return null;
      return {
        from: item.path,
        fromSource: (item.source || state.req.source) as string,
        name: item.name,
        isDir: item.type === "directory",
      };
    })
    .filter((entry): entry is ClipboardEntry => entry !== null);

  if (clipboardItems.length === 0) {
    return;
  }

  clipboard = {
    key: key,
    items: clipboardItems,
    path: state.route.path,
  };
  internalClipboardTimestamp = Date.now();
}

async function handleInternalPaste() {
  if (!clipboard?.items || clipboard.items.length === 0) {
    return;
  }

  const operation = clipboard.key === "x" ? "move" : "copy";
  // Pasting writes into the current source; a cut also removes the items from where they were cut,
  // and a share refuses any change unless it allows modify.
  const canPaste =
    getters.sourcePermissions().create &&
    clipboard.items.every((item) => {
      const origin = getters.sourcePermissions(item.fromSource);
      return operation === "move" || getters.isShare() ? origin.modify : origin.download;
    });
  if (!canPaste) {
    return;
  }

  // Construct destination path properly (without URL prefix)
  const reqPath = state.req.path as string;
  const destPath = reqPath.endsWith("/") ? reqPath : `${reqPath}/`;

  const items = clipboard.items.map((item) => ({
    from: item.from,
    fromSource: item.fromSource,
    to: destPath + item.name,
    toSource: state.req.source,
    name: item.name,
    isDir: item.isDir,
  }));

  // Show confirmation prompt first
  mutations.showPrompt({
    name: "CopyPasteConfirm",
    props: {
      operation: operation,
      items: items,
      onConfirm: () => {
        return new Promise<void>((resolve, reject) => {

          const action = async (overwrite: boolean, rename: boolean) => {
            try {
              if (getters.isShare()) {
                await resourcesApi.moveCopyPublic(state.shareInfo.hash, items, operation, overwrite, rename);
              } else {
                await resourcesApi.moveCopy(items, operation, overwrite, rename);
              }
              if (operation === "move") {
                clipboard = { items: [] };
                internalClipboardTimestamp = 0;
              }
              mutations.setReload(true);
              resolve();
            } catch (error) {
              notifyMoveCopyFailure(error);
              reject(error);
            }
          };

          if (clipboard.path === state.route.path) {
            void action(false, true);
            return;
          }

          const conflict = checkConflict(items, state.req.items);

          if (conflict) {
            mutations.showPrompt({
              name: "replace-rename",
              pinned: true,
              confirm: (event: Event, option: string) => {
                const overwrite = option === "overwrite";
                const rename = option === "rename";
                event.preventDefault();
                mutations.closeTopPrompt();
                void action(overwrite, rename);
              },
            });
            return;
          }
          void action(false, false);
        });
      },
    },
  });
}

export function useListingClipboard() {
  return { copyCut, handleInternalPaste, hasInternalItems, isInternalRecent };
}
