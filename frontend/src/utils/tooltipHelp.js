import { getters, mutations, state } from "@/store";

let dismissHandler = null;
let dismissTimer = null;
let tapCooldown = false;
let tapCooldownTimer = null;
let globalDismissListenersRegistered = false;

function onGlobalDismiss() {
  if (state.tooltip.show) {
    hideInteractiveTooltip(true);
  }
}

function ensureGlobalDismissListeners() {
  if (globalDismissListenersRegistered || typeof document === "undefined") {
    return;
  }
  globalDismissListenersRegistered = true;
  document.addEventListener("scroll", onGlobalDismiss, {
    capture: true,
    passive: true,
  });
  window.addEventListener("popstate", onGlobalDismiss);
  window.addEventListener("hashchange", onGlobalDismiss);
}

ensureGlobalDismissListeners();

export function useTapForTooltip() {
  if (getters.isMobile()) {
    return true;
  }
  if (typeof window !== "undefined" && window.matchMedia) {
    return window.matchMedia("(hover: none)").matches;
  }
  return false;
}

export function tooltipEventCoords(event) {
  const touch = event?.changedTouches?.[0] || event?.touches?.[0];
  if (touch) {
    return { x: touch.clientX, y: touch.clientY };
  }
  if (typeof event?.clientX === "number" && typeof event?.clientY === "number") {
    return { x: event.clientX, y: event.clientY };
  }
  if (event?.currentTarget instanceof Element) {
    const rect = event.currentTarget.getBoundingClientRect();
    return {
      x: rect.left + rect.width / 2,
      y: rect.top + rect.height / 2,
    };
  }
  return { x: 0, y: 0 };
}

function isTooltipTrigger(target) {
  return (
    target instanceof Element &&
    target.closest(".tooltip-info-icon, .floating-tooltip")
  );
}

function isTapOpenedTooltip() {
  return state.tooltip.show && state.tooltip.pointerEvents;
}

function startTapCooldown() {
  tapCooldown = true;
  if (tapCooldownTimer) {
    clearTimeout(tapCooldownTimer);
  }
  tapCooldownTimer = setTimeout(() => {
    tapCooldown = false;
    tapCooldownTimer = null;
  }, 300);
}

export function shouldIgnoreOutsideTap() {
  return tapCooldown;
}

function unregisterTooltipDismiss() {
  if (dismissTimer) {
    clearTimeout(dismissTimer);
    dismissTimer = null;
  }
  if (!dismissHandler) {
    return;
  }
  document.removeEventListener("click", dismissHandler);
  dismissHandler = null;
}

function registerTooltipDismiss() {
  unregisterTooltipDismiss();
  dismissHandler = (event) => {
    if (tapCooldown || isTooltipTrigger(event.target)) {
      return;
    }
    hideInteractiveTooltip(true);
  };
  dismissTimer = setTimeout(() => {
    dismissTimer = null;
    if (dismissHandler) {
      document.addEventListener("click", dismissHandler);
    }
  }, 0);
}

export function hideInteractiveTooltip(force = false) {
  if (!force && useTapForTooltip() && isTapOpenedTooltip()) {
    return;
  }
  unregisterTooltipDismiss();
  mutations.hideTooltip();
}

export function showHoverTooltip(content, event) {
  const { x, y } = tooltipEventCoords(event);
  mutations.showTooltip({
    content,
    x,
    y,
    pointerEvents: false,
  });
}

function showTapTooltip(content, event) {
  const { x, y } = tooltipEventCoords(event);
  mutations.showTooltip({
    content,
    x,
    y,
    pointerEvents: true,
  });
  registerTooltipDismiss();
}

/** @deprecated Use showHoverTooltip or tap handlers instead */
export function showInteractiveTooltip(content, event) {
  if (useTapForTooltip()) {
    showTapTooltip(content, event);
    return;
  }
  showHoverTooltip(content, event);
}

export function isTooltipContentVisible(content) {
  return (
    state.tooltip.show &&
    !state.tooltip.component &&
    state.tooltip.content === content
  );
}

export function onTooltipHelpMouseEnter(event, content) {
  if (tapCooldown || isTapOpenedTooltip()) {
    return;
  }
  showHoverTooltip(content, event);
}

export function onTooltipHelpMouseLeave() {
  if (isTapOpenedTooltip()) {
    return;
  }
  hideInteractiveTooltip(true);
}

export function onTooltipHelpTouchEnd(event, content) {
  if (!useTapForTooltip()) {
    return;
  }
  event.stopPropagation();
  startTapCooldown();
  unregisterTooltipDismiss();
  showTapTooltip(content, event);
}

export function onTooltipHelpClick(event, content) {
  event.stopPropagation();
  if (useTapForTooltip()) {
    return;
  }
  showHoverTooltip(content, event);
}

export function isComponentTooltipVisible(component, propsMatcher = () => true) {
  if (!state.tooltip.show || state.tooltip.component !== component) {
    return false;
  }
  return propsMatcher(state.tooltip.componentProps ?? {});
}

export function showHoverComponentTooltip({
  component,
  componentProps,
  event,
  width,
}) {
  const { x, y } = tooltipEventCoords(event);
  mutations.showTooltip({
    component,
    componentProps,
    x,
    y,
    width: width ?? null,
    pointerEvents: false,
  });
}

function showTapComponentTooltip({ component, componentProps, event, width }) {
  const { x, y } = tooltipEventCoords(event);
  mutations.showTooltip({
    component,
    componentProps,
    x,
    y,
    width: width ?? null,
    pointerEvents: true,
  });
  registerTooltipDismiss();
}

export function showInteractiveComponentTooltip({
  component,
  componentProps,
  event,
  pointerEvents,
  width,
}) {
  const tap = pointerEvents ?? useTapForTooltip();
  if (tap) {
    showTapComponentTooltip({ component, componentProps, event, width });
    return;
  }
  showHoverComponentTooltip({ component, componentProps, event, width });
}

export function onComponentTooltipTouchEnd(options) {
  if (!useTapForTooltip()) {
    return;
  }
  const { event } = options;
  event.stopPropagation();
  startTapCooldown();
  unregisterTooltipDismiss();
  showTapComponentTooltip(options);
}

export function onComponentTooltipClick(options) {
  const { event } = options;
  event.stopPropagation();
  if (useTapForTooltip()) {
    return;
  }
  showHoverComponentTooltip(options);
}
