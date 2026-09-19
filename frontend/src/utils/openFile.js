import { resourcesApi } from "@/api";
import { ensureViewToken, getCachedViewToken } from "@/api/viewToken.js";
import { notify } from "@/notify";
import { mutations, state } from "@/store";
import { openUrlInNewTab } from "@/utils/inlineOpen.js";
import { isPdfFile } from "@/utils/mediaFile";
import { shouldUsePdfPreviewFallback } from "@/utils/pdfPreview.js";

/** Navigate to a view URL in the same tab (mobile Chromium PDF open). */
export function openPdfViewInSameTab(url) {
  if (!url) {
    return;
  }
  window.location.assign(url);
}

/**
 * Open the current file in a new tab for inline viewing (PDFs use /resources/view + viewToken).
 */
export async function openFileInNewTab({
  source,
  path,
  shareInfo = null,
  mimeOrName = "",
}) {
  const typeHint = mimeOrName || state.req?.type || state.req?.name || path;
  const isPdf = isPdfFile(typeHint) || isPdfFile(path);

  let viewToken = state.req?.viewToken || getCachedViewToken(source);

  if (isPdf) {
    if (!viewToken && source) {
      try {
        viewToken = await ensureViewToken(source);
        if (viewToken) {
          mutations.setRequestViewToken(viewToken);
        }
      } catch (err) {
        console.warn("Failed to mint view token for open file:", err);
      }
    }
    if (!viewToken) {
      notify.showError("Could not open PDF — try previewing the file first.");
      return;
    }
  }

  const openUrl = resourcesApi.getOpenFileURL(source, path, shareInfo, {
    viewToken,
    mimeOrName: typeHint,
  });
  if (!openUrl) {
    return;
  }
  if (isPdf && shouldUsePdfPreviewFallback()) {
    openPdfViewInSameTab(openUrl);
    return;
  }
  openUrlInNewTab(openUrl);
}
