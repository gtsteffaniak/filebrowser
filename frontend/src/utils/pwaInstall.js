import { ref } from "vue";

let deferredPrompt = null;
let initialized = false;

/** Reactive flag for Vue components. */
export const installAvailable = ref(false);

function syncAvailability() {
  installAvailable.value = deferredPrompt !== null && !isStandalone();
}

export function isStandalone() {
  if (typeof window === "undefined") {
    return false;
  }
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    window.navigator.standalone === true
  );
}

export function canInstall() {
  return installAvailable.value;
}

function storeDeferredPrompt(event) {
  if (!event) {
    return;
  }
  event.preventDefault();
  deferredPrompt = event;
  window.__pwaDeferredPrompt = event;
  syncAvailability();
}

export function initPwaInstall() {
  if (initialized || typeof window === "undefined") {
    return;
  }
  initialized = true;

  // Recover prompt captured by inline script in index.html before the bundle loaded.
  if (window.__pwaDeferredPrompt) {
    storeDeferredPrompt(window.__pwaDeferredPrompt);
  }

  window.addEventListener("beforeinstallprompt", storeDeferredPrompt);

  window.addEventListener("pwa-install-available", () => {
    if (window.__pwaDeferredPrompt) {
      storeDeferredPrompt(window.__pwaDeferredPrompt);
    }
  });

  window.addEventListener("appinstalled", () => {
    deferredPrompt = null;
    window.__pwaDeferredPrompt = null;
    syncAvailability();
  });

  syncAvailability();
}

export async function promptInstall() {
  if (!deferredPrompt) {
    return false;
  }

  const promptEvent = deferredPrompt;
  deferredPrompt = null;
  window.__pwaDeferredPrompt = null;
  syncAvailability();

  await promptEvent.prompt();
  const { outcome } = await promptEvent.userChoice;
  return outcome === "accepted";
}
