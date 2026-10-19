import type { RouteLocationNormalized, Router } from "vue-router";
import { mutations, state } from "@/store";

let approvedNavigationFullPath: string | null = null;

export function approveUploadNavigation(fullPath: string) {
  approvedNavigationFullPath = fullPath;
}

export function clearApprovedUploadNavigation() {
  approvedNavigationFullPath = null;
}

/** One-time bypass for the exact destination the user confirmed. */
export function consumeApprovedUploadNavigation(
  to: RouteLocationNormalized
): boolean {
  if (approvedNavigationFullPath === null) {
    return false;
  }
  if (to.fullPath === approvedNavigationFullPath) {
    approvedNavigationFullPath = null;
    return true;
  }
  return false;
}

export function hasCloseWithActiveUploadsPrompt(): boolean {
  return state.prompts.some((p) => p.name === "CloseWithActiveUploads");
}

export function dismissCloseWithActiveUploadsPrompt() {
  const warnings = state.prompts.filter(
    (p) => p.name === "CloseWithActiveUploads"
  );
  for (const warning of warnings) {
    mutations.closeTopPrompt(warning.id, { force: true });
  }
}

export function isDifferentAppRoute(
  to: RouteLocationNormalized,
  from: RouteLocationNormalized
): boolean {
  return (
    to.path !== from.path ||
    to.hash !== from.hash ||
    JSON.stringify(to.query) !== JSON.stringify(from.query)
  );
}

export type UploadNavigationGuardResult = "allow" | "block" | "prompt";

export function evaluateUploadNavigationGuard(
  to: RouteLocationNormalized
): UploadNavigationGuardResult {
  if (!state.upload.isUploading) {
    clearApprovedUploadNavigation();
    return "allow";
  }
  if (consumeApprovedUploadNavigation(to)) {
    return "allow";
  }
  if (hasCloseWithActiveUploadsPrompt()) {
    return "block";
  }
  return "prompt";
}

export function showUploadNavigationConfirm(
  to: RouteLocationNormalized,
  router: Router
) {
  const uploadPrompt = state.prompts.find((p) => p.name === "upload");
  mutations.showPrompt({
    name: "CloseWithActiveUploads",
    pinned: true,
    confirm: () => {
      approveUploadNavigation(to.fullPath);
      dismissCloseWithActiveUploadsPrompt();
      if (uploadPrompt) {
        mutations.closeTopPrompt(uploadPrompt.id, { force: true });
      }
      void router.push(to).finally(() => {
        clearApprovedUploadNavigation();
      });
    },
    cancel: () => {
      clearApprovedUploadNavigation();
      dismissCloseWithActiveUploadsPrompt();
    },
  });
}

export function confirmCloseUploadPrompt(uploadPromptId?: number) {
  dismissCloseWithActiveUploadsPrompt();
  if (uploadPromptId !== undefined) {
    mutations.closeTopPrompt(uploadPromptId, { force: true });
  }
}

export function cancelCloseUploadPrompt() {
  clearApprovedUploadNavigation();
  dismissCloseWithActiveUploadsPrompt();
}
