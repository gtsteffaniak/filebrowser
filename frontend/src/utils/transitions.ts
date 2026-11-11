const POP_SCALE_X = 0.75;
const POP_SCALE_Y = 0.5625;
const POP_EASING_IN = "cubic-bezier(0.4, 0, 0.2, 1)";
const POP_EASING_OUT = "cubic-bezier(0.4, 0, 1, 1)";

/** `scale` val for the progress: 0 = collapsed, 1 = fully shown */
function popScale(progress: number, fromScaleX: number): string {
  return `${fromScaleX + (1 - fromScaleX) * progress} ${POP_SCALE_Y + (1 - POP_SCALE_Y) * progress}`;
}

// Running animation for each element
const running = new WeakMap<Element, Animation>();
let reducedMotionQuery: MediaQueryList | null | undefined;

function prefersReducedMotion(): boolean {
  if (reducedMotionQuery === undefined) {
    reducedMotionQuery =
      typeof window !== "undefined" && window.matchMedia
        ? window.matchMedia("(prefers-reduced-motion: reduce)")
        : null;
  }
  return !!reducedMotionQuery?.matches;
}

interface AnimateOptions {
  /** Set while the animation runs */
  willChange?: string;
  /** Keeps the end state after finishing until the element is removed */
  fill?: FillMode;
  easing: string;
}

function animate(
  el: HTMLElement,
  from: Keyframe,
  to: Keyframe,
  durationMs: number,
  done: () => void,
  { willChange, fill, easing }: AnimateOptions,
): void {
  running.get(el)?.cancel();
  let finished = false;
  const finish = () => {
    if (finished) return;
    finished = true;
    done();
  };

  const reduceMotion = prefersReducedMotion();
  if (typeof el.animate !== "function" || reduceMotion) {
    setTimeout(finish, reduceMotion ? 0 : durationMs);
    return;
  }

  if (willChange) el.style.willChange = willChange;
  const animation = el.animate([from, to], { duration: durationMs, easing, fill });
  running.set(el, animation);
  const release = () => {
    if (running.get(el) === animation) {
      running.delete(el);
      if (willChange) el.style.willChange = "";
    }
    finish();
    if (fill) {
      animation.oncancel = null;
      animation.cancel();
    }
  };
  animation.onfinish = release;
  animation.oncancel = release;
}

function runningProgress(el: Element): number | null {
  const animation = running.get(el);
  if (!animation) return null;
  const progress = animation.effect?.getComputedTiming().progress;
  return typeof progress === "number" ? progress : 0;
}

export function popEnter(node: Element, done: () => void, durationMs = 200, fromScaleX = POP_SCALE_X): void {
  const el = node as HTMLElement;
  el.style.opacity = "";
  animate(el, { opacity: 0, scale: popScale(0, fromScaleX) }, { opacity: 1, scale: popScale(1, fromScaleX) }, durationMs, done, {
    willChange: "opacity, scale",
    easing: POP_EASING_IN,
  });
}

export function popLeave(node: Element, done: () => void, durationMs = 120, fromScaleX = POP_SCALE_X): void {
  const el = node as HTMLElement;
  const progress = runningProgress(el) ?? 1;
  el.style.opacity = "0";
  animate(
    el,
    { opacity: progress, scale: popScale(progress, fromScaleX) },
    { opacity: 0, scale: popScale(0, fromScaleX) },
    durationMs,
    () => {
      done();
      el.style.opacity = "";
    },
    { willChange: "opacity, scale", fill: "forwards", easing: POP_EASING_OUT },
  );
}

const SLIDE_PX = 8;

function slideTranslate(progress: number): string {
  return `0 ${-SLIDE_PX * (1 - progress)}px`;
}

export function slideEnter(node: Element, done: () => void, durationMs = 180): void {
  const el = node as HTMLElement;
  el.style.opacity = "";
  animate(el, { opacity: 0, translate: slideTranslate(0) }, { opacity: 1, translate: slideTranslate(1) }, durationMs, done, {
    willChange: "opacity, translate",
    easing: POP_EASING_IN,
  });
}

export function slideLeave(node: Element, done: () => void, durationMs = 120): void {
  const el = node as HTMLElement;
  const progress = runningProgress(el) ?? 1;
  el.style.opacity = "0";
  animate(
    el,
    { opacity: progress, translate: slideTranslate(progress) },
    { opacity: 0, translate: slideTranslate(0) },
    durationMs,
    () => {
      done();
      el.style.opacity = "";
    },
    { willChange: "opacity, translate", fill: "forwards", easing: POP_EASING_OUT },
  );
}

// Collapse height, vertical padding, margin and opacity go to zero together
// (this is used in the collapsibles in settings. (but is a bit expensive)
const COLLAPSE_MS_IN = 200;
const COLLAPSE_MS_OUT = 160;

function collapsedFrame(opacity: number): Keyframe {
  return { height: "0px", paddingTop: "0px", paddingBottom: "0px", marginTop: "0px", marginBottom: "0px", opacity };
}

/** The element as is rn */
function currentFrame(el: HTMLElement): Keyframe {
  const style = getComputedStyle(el);
  return {
    height: style.height,
    paddingTop: style.paddingTop,
    paddingBottom: style.paddingBottom,
    marginTop: style.marginTop,
    marginBottom: style.marginBottom,
    opacity: style.opacity,
  };
}

function collapse(el: HTMLElement, from: Keyframe, to: Keyframe, durationMs: number, done: () => void, fill?: FillMode): void {
  el.style.overflow = "hidden";
  animate(
    el,
    from,
    to,
    durationMs,
    () => {
      done();
      if (!running.has(el)) el.style.overflow = "";
    },
    { fill, easing: POP_EASING_IN },
  );
}

export function collapseEnter(node: Element, done: () => void, durationMs = COLLAPSE_MS_IN): void {
  const el = node as HTMLElement;
  el.style.opacity = "";
  running.get(el)?.cancel();
  collapse(el, collapsedFrame(0), { ...currentFrame(el), opacity: 1 }, durationMs, done);
}

export function collapseLeave(node: Element, done: () => void, durationMs = COLLAPSE_MS_OUT): void {
  const el = node as HTMLElement;
  const from = currentFrame(el);
  collapse(el, from, collapsedFrame(0), durationMs, done, "forwards");
}
