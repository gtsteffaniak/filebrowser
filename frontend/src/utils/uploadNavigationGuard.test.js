import { beforeEach, describe, expect, it, vi } from "vitest";

const { closeTopPrompt } = vi.hoisted(() => ({
  closeTopPrompt: vi.fn(),
}));

vi.mock("@/store", () => ({
  state: {
    upload: { isUploading: true },
    prompts: [],
  },
  mutations: {
    closeTopPrompt,
    showPrompt: vi.fn(),
  },
}));

import { state } from "@/store";
import {
  approveUploadNavigation,
  cancelCloseUploadPrompt,
  clearApprovedUploadNavigation,
  consumeApprovedUploadNavigation,
  dismissCloseWithActiveUploadsPrompt,
  evaluateUploadNavigationGuard,
  hasCloseWithActiveUploadsPrompt,
} from "./uploadNavigationGuard";

const to = (fullPath) => ({ fullPath, path: fullPath, hash: "", query: {} });

beforeEach(() => {
  clearApprovedUploadNavigation();
  state.upload.isUploading = true;
  state.prompts = [];
  closeTopPrompt.mockClear();
});

describe("uploadNavigationGuard", () => {
  it("consumes a one-time approval for the matching fullPath", () => {
    approveUploadNavigation("/files/docs/");
    expect(consumeApprovedUploadNavigation(to("/files/docs/"))).toBe(true);
    expect(consumeApprovedUploadNavigation(to("/files/docs/"))).toBe(false);
  });

  it("blocks navigation while the confirmation prompt is open", () => {
    state.prompts = [{ id: 2, name: "CloseWithActiveUploads" }];
    expect(hasCloseWithActiveUploadsPrompt()).toBe(true);
    expect(evaluateUploadNavigationGuard(to("/files/other/"))).toBe("block");
  });

  it("clears approval on cancel helper", () => {
    approveUploadNavigation("/files/a/");
    state.prompts = [{ id: 9, name: "CloseWithActiveUploads" }];
    cancelCloseUploadPrompt();
    expect(closeTopPrompt).toHaveBeenCalledWith(9, { force: true });
    state.prompts = [];
    expect(evaluateUploadNavigationGuard(to("/files/a/"))).toBe("prompt");
  });

  it("does not reuse approval after uploads finish", () => {
    approveUploadNavigation("/files/same/");
    state.upload.isUploading = false;
    expect(evaluateUploadNavigationGuard(to("/files/same/"))).toBe("allow");
    state.upload.isUploading = true;
    expect(evaluateUploadNavigationGuard(to("/files/same/"))).toBe("prompt");
  });

  it("dismisses all CloseWithActiveUploads prompts", () => {
    state.prompts = [
      { id: 1, name: "upload" },
      { id: 2, name: "CloseWithActiveUploads" },
    ];
    dismissCloseWithActiveUploadsPrompt();
    expect(closeTopPrompt).toHaveBeenCalledWith(2, { force: true });
  });
});
