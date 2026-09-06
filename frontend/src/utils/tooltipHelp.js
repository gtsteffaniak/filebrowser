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
  if (event?.clientX != null && event?.clientY != null) {
    return { x: event.clientX, y: event.clientY };
  }
  const touch = event?.changedTouches?.[0] || event?.touches?.[0];
  if (touch) {
    return { x: touch.clientX, y: touch.clientY };
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

export function showInteractiveTooltip(content, event) {
  const tap = useTapForTooltip();
  const { x, y } = tooltipEventCoords(event);
  mutations.showTooltip({
    content,
    x,
    y,
    pointerEvents: tap,
  });
  if (tap) {
    registerTooltipDismiss();
  }
}

export function isTooltipContentVisible(content) {
  return (
    state.tooltip.show &&
    !state.tooltip.component &&
    state.tooltip.content === content
  );
}

export function onTooltipHelpMouseEnter(event, content) {
  if (useTapForTooltip()) {
    return;
  }
  showInteractiveTooltip(content, event);
}

export function onTooltipHelpMouseLeave() {
  if (useTapForTooltip()) {
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
  showInteractiveTooltip(content, event);
}

export function onTooltipHelpClick(event, content) {
  event.stopPropagation();
  if (useTapForTooltip()) {
    return;
  }
  showInteractiveTooltip(content, event);
}

export function isComponentTooltipVisible(component, propsMatcher = () => true) {
  if (!state.tooltip.show || state.tooltip.component !== component) {
    return false;
  }
  return propsMatcher(state.tooltip.componentProps ?? {});
}

export function showInteractiveComponentTooltip({
  component,
  componentProps,
  event,
  pointerEvents,
  width,
}) {
  const tap = pointerEvents ?? useTapForTooltip();
  const { x, y } = tooltipEventCoords(event);
  mutations.showTooltip({
    component,
    componentProps,
    x,
    y,
    width: width ?? null,
    pointerEvents: tap,
  });
  if (tap) {
    registerTooltipDismiss();
  }
}

export function onComponentTooltipTouchEnd(options) {
  if (!useTapForTooltip()) {
    return;
  }
  const { event } = options;
  event.stopPropagation();
  startTapCooldown();
  unregisterTooltipDismiss();
  showInteractiveComponentTooltip(options);
}

export function onComponentTooltipClick(options) {
  const { event } = options;
  event.stopPropagation();
  if (useTapForTooltip()) {
    return;
  }
  showInteractiveComponentTooltip(options);
}
