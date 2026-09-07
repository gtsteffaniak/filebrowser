import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: {
      tooltip: {
        show: false,
        content: "",
        component: null,
        pointerEvents: false,
      },
    },
    getters: {
      isMobile: vi.fn(() => false),
    },
    mutations: {
      showTooltip: vi.fn(),
      hideTooltip: vi.fn(),
    },
  },
}));

vi.mock("@/store", () => storeMock);

import {
  hideInteractiveTooltip,
  isTooltipContentVisible,
  onTooltipHelpClick,
  onTooltipHelpMouseEnter,
  onTooltipHelpMouseLeave,
  onTooltipHelpTouchEnd,
  shouldIgnoreOutsideTap,
  showHoverTooltip,
  showInteractiveTooltip,
  tooltipEventCoords,
} from "./tooltipHelp.js";

describe("tooltipHelp", () => {
  beforeEach(() => {
    storeMock.getters.isMobile.mockReturnValue(false);
    storeMock.state.tooltip.show = false;
    storeMock.state.tooltip.content = "";
    storeMock.state.tooltip.component = null;
    storeMock.state.tooltip.pointerEvents = false;
    storeMock.mutations.showTooltip.mockClear();
    storeMock.mutations.hideTooltip.mockClear();
    hideInteractiveTooltip(true);
    storeMock.mutations.showTooltip.mockClear();
    storeMock.mutations.hideTooltip.mockClear();
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false })));
  });

  afterEach(() => {
    hideInteractiveTooltip(true);
    vi.unstubAllGlobals();
  });

  it("uses touch coordinates from touchend events", () => {
    const coords = tooltipEventCoords({
      changedTouches: [{ clientX: 42, clientY: 84 }],
    });
    expect(coords).toEqual({ x: 42, y: 84 });
  });

  it("prefers touch coordinates when clientX is undefined", () => {
    const coords = tooltipEventCoords({
      clientX: undefined,
      clientY: undefined,
      changedTouches: [{ clientX: 10, clientY: 20 }],
    });
    expect(coords).toEqual({ x: 10, y: 20 });
  });

  it("uses mouse coordinates for hover events", () => {
    const coords = tooltipEventCoords({ clientX: 15, clientY: 25 });
    expect(coords).toEqual({ x: 15, y: 25 });
  });

  it("shows tooltip on desktop hover", () => {
    const event = { clientX: 10, clientY: 20 };
    onTooltipHelpMouseEnter(event, "help text");
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledWith({
      content: "help text",
      x: 10,
      y: 20,
      pointerEvents: false,
    });
  });

  it("shows hover tooltip even when tap mode is enabled", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true })));
    const event = { clientX: 10, clientY: 20 };
    onTooltipHelpMouseEnter(event, "help text");
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledWith({
      content: "help text",
      x: 10,
      y: 20,
      pointerEvents: false,
    });
  });

  it("hides tooltip on desktop mouse leave", () => {
    onTooltipHelpMouseLeave();
    expect(storeMock.mutations.hideTooltip).toHaveBeenCalled();
  });

  it("shows tooltip on mobile tap and keeps it open on repeat tap", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    const event = {
      clientX: 5,
      clientY: 6,
      preventDefault: vi.fn(),
      stopPropagation: vi.fn(),
    };

    onTooltipHelpTouchEnd(event, "mobile help");
    expect(event.stopPropagation).toHaveBeenCalled();
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledWith({
      content: "mobile help",
      x: 5,
      y: 6,
      pointerEvents: true,
    });

    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.content = "mobile help";
    storeMock.state.tooltip.pointerEvents = true;
    storeMock.mutations.hideTooltip.mockClear();
    storeMock.mutations.showTooltip.mockClear();

    onTooltipHelpTouchEnd(event, "mobile help");
    expect(storeMock.mutations.hideTooltip).not.toHaveBeenCalled();
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledTimes(1);
  });

  it("ignores the synthetic click after a mobile tap", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    const touchEvent = {
      clientX: 8,
      clientY: 9,
      stopPropagation: vi.fn(),
    };
    const clickEvent = {
      clientX: 8,
      clientY: 9,
      stopPropagation: vi.fn(),
    };

    onTooltipHelpTouchEnd(touchEvent, "touch help");
    onTooltipHelpClick(clickEvent, "touch help");

    expect(storeMock.mutations.showTooltip).toHaveBeenCalledTimes(1);
    expect(shouldIgnoreOutsideTap()).toBe(true);
  });

  it("tracks visible tooltip content", () => {
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.content = "visible";
    expect(isTooltipContentVisible("visible")).toBe(true);
    expect(isTooltipContentVisible("other")).toBe(false);
  });

  it("ignores hover-driven hide while a tap tooltip is open", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    showInteractiveTooltip("help", { clientX: 1, clientY: 2 });
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.pointerEvents = true;
    storeMock.mutations.hideTooltip.mockClear();

    hideInteractiveTooltip();
    expect(storeMock.mutations.hideTooltip).not.toHaveBeenCalled();

    hideInteractiveTooltip(true);
    expect(storeMock.mutations.hideTooltip).toHaveBeenCalled();
  });

  it("hides tooltip on scroll", () => {
    storeMock.state.tooltip.show = true;
    document.dispatchEvent(new Event("scroll"));
    expect(storeMock.mutations.hideTooltip).toHaveBeenCalled();
  });

  it("hides tooltip on browser navigation", () => {
    storeMock.state.tooltip.show = true;
    window.dispatchEvent(new Event("popstate"));
    expect(storeMock.mutations.hideTooltip).toHaveBeenCalled();
  });

  it("ignores synthetic mouseleave while a tap tooltip is open", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    onTooltipHelpTouchEnd(
      { clientX: 1, clientY: 2, stopPropagation: vi.fn() },
      "tap help",
    );
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.pointerEvents = true;
    storeMock.mutations.hideTooltip.mockClear();

    onTooltipHelpMouseLeave();
    expect(storeMock.mutations.hideTooltip).not.toHaveBeenCalled();
  });

  it("does not replace a tap tooltip with a hover tooltip on mouseenter", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    onTooltipHelpTouchEnd(
      { clientX: 1, clientY: 2, stopPropagation: vi.fn() },
      "tap help",
    );
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.pointerEvents = true;
    storeMock.mutations.showTooltip.mockClear();

    onTooltipHelpMouseEnter({ clientX: 3, clientY: 4 }, "hover help");
    expect(storeMock.mutations.showTooltip).not.toHaveBeenCalled();
  });

  it("switches tap tooltip when tapping another icon", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    const firstEvent = {
      clientX: 5,
      clientY: 6,
      stopPropagation: vi.fn(),
    };
    const secondEvent = {
      clientX: 20,
      clientY: 30,
      stopPropagation: vi.fn(),
    };

    onTooltipHelpTouchEnd(firstEvent, "first help");
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.content = "first help";
    storeMock.state.tooltip.pointerEvents = true;
    onTooltipHelpMouseLeave();

    storeMock.mutations.showTooltip.mockClear();
    storeMock.mutations.hideTooltip.mockClear();

    onTooltipHelpTouchEnd(secondEvent, "second help");
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.pointerEvents = true;
    onTooltipHelpMouseLeave();

    expect(storeMock.mutations.hideTooltip).not.toHaveBeenCalled();
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledWith({
      content: "second help",
      x: 20,
      y: 30,
      pointerEvents: true,
    });
  });

  it("switches tooltip when tapping a different help icon", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    const firstEvent = {
      clientX: 5,
      clientY: 6,
      stopPropagation: vi.fn(),
    };
    const secondEvent = {
      clientX: 20,
      clientY: 30,
      stopPropagation: vi.fn(),
    };

    onTooltipHelpTouchEnd(firstEvent, "first help");
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.content = "first help";
    storeMock.state.tooltip.pointerEvents = true;
    storeMock.mutations.showTooltip.mockClear();

    onTooltipHelpTouchEnd(secondEvent, "second help");
    storeMock.state.tooltip.show = true;
    storeMock.state.tooltip.pointerEvents = true;
    storeMock.mutations.hideTooltip.mockClear();
    hideInteractiveTooltip();
    expect(storeMock.mutations.hideTooltip).not.toHaveBeenCalled();
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledWith({
      content: "second help",
      x: 20,
      y: 30,
      pointerEvents: true,
    });
  });

  it("showHoverTooltip always uses hover mode", () => {
    storeMock.getters.isMobile.mockReturnValue(true);
    showHoverTooltip("hover help", { clientX: 3, clientY: 4 });
    expect(storeMock.mutations.showTooltip).toHaveBeenCalledWith({
      content: "hover help",
      x: 3,
      y: 4,
      pointerEvents: false,
    });
  });
});
