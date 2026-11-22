import { shallowRef } from "vue";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type Plyr from "plyr";

const { gettersMock, stateMock } = vi.hoisted(() => ({
  gettersMock: { isMobile: vi.fn(() => false) },
  stateMock: { prompts: [] as unknown[] },
}));
vi.mock("@/store/index.ts", () => ({ getters: gettersMock, state: stateMock }));

import { useMediaKeyboard } from "../../player/useMediaKeyboard.ts";

function setup(previewType = "video") {
  const opts = {
    player: shallowRef({} as Plyr),
    getPreviewType: () => previewType,
    cyclePlaybackModes: vi.fn(),
    toggleSingleLoop: vi.fn(),
    showQueuePrompt: vi.fn(),
    flashSkipFeedback: vi.fn(),
    toggleDesktopPanel: vi.fn(),
  };
  const { attachKeyboard, detachKeyboard } = useMediaKeyboard(opts);
  attachKeyboard();
  return { opts, detachKeyboard };
}

const press = (key: string, init: KeyboardEventInit = {}) => {
  document.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init }));
};

describe("useMediaKeyboard", () => {
  beforeEach(() => {
    stateMock.prompts = [];
    gettersMock.isMobile.mockReturnValue(false);
  });

  it("maps P, L and arrows to their actions", () => {
    const { opts, detachKeyboard } = setup();
    press("p");
    press("L");
    press("ArrowLeft");
    press("ArrowRight");
    expect(opts.cyclePlaybackModes).toHaveBeenCalledTimes(1);
    expect(opts.toggleSingleLoop).toHaveBeenCalledTimes(1);
    expect(opts.flashSkipFeedback.mock.calls).toEqual([[true], [false]]);
    detachKeyboard();
  });

  it("Q toggles the side panel on desktop audio and opens the queue otherwise", () => {
    const audio = setup("audio");
    press("q");
    expect(audio.opts.toggleDesktopPanel).toHaveBeenCalledTimes(1);
    expect(audio.opts.showQueuePrompt).not.toHaveBeenCalled();
    audio.detachKeyboard();

    const video = setup("video");
    press("q");
    expect(video.opts.showQueuePrompt).toHaveBeenCalledTimes(1);
    video.detachKeyboard();
  });

  it("ignores modifiers, repeats, form fields and open prompts", () => {
    const { opts, detachKeyboard } = setup();
    press("p", { ctrlKey: true });
    press("p", { repeat: true });
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "p", bubbles: true }));
    stateMock.prompts = [{}];
    press("q");
    expect(opts.cyclePlaybackModes).not.toHaveBeenCalled();
    expect(opts.showQueuePrompt).not.toHaveBeenCalled();
    detachKeyboard();
  });

  it("stops listening after detach", () => {
    const { opts, detachKeyboard } = setup();
    detachKeyboard();
    press("p");
    expect(opts.cyclePlaybackModes).not.toHaveBeenCalled();
  });
});
