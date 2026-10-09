<template>
  <div class="epub-container">
    <div v-if="!isReady" class="loading-indicator">
      <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
    </div>

    <div id="viewer" ref="viewer" :class="{ ready: isReady }"></div>

    <div v-if="isReady" class="navigation">
      <button type="button" @click="prevPage" class="nav-button">&lt;</button> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <button type="button" @click="nextPage" class="nav-button">&gt;</button> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { Book, Rendition } from "@likecoin/epub-ts";
import { state, mutations, getters } from "@/store";
import { resourcesApi } from "@/api";
import { ensureViewToken, requestViewIdentity, getCachedViewToken, getRequestViewToken } from "@/api/viewToken.js";

defineOptions({ name: "epubViewer" });

/** Hash format: `#epubcfi=<encodeURIComponent(epub-cfi)>` — distinct from listing `#filename` hashes. */
const EPUB_HASH_PREFIX = "epubcfi=";

function parseEpubCfiFromHash(): string | null {
  const h = window.location.hash;
  if (!h || h.length <= 1) return null;
  const raw = h.slice(1);
  if (!raw.startsWith(EPUB_HASH_PREFIX)) return null;
  try {
    return decodeURIComponent(raw.slice(EPUB_HASH_PREFIX.length));
  } catch {
    return null;
  }
}

function replaceUrlHashWithEpubCfi(cfi: string) {
  const newHash = `#${EPUB_HASH_PREFIX}${encodeURIComponent(cfi)}`;
  history.replaceState(null, "", `${window.location.pathname}${window.location.search}${newHash}`);
}

function cfiToString(cfi: unknown): string {
  if (typeof cfi === "string") return cfi;
  if (cfi !== null && typeof (cfi as { toString?: () => string }).toString === "function") {
    return String((cfi as { toString: () => string }).toString());
  }
  return "";
}

const isReady = ref(false); // to indicate when the book is loaded
const floatIn = ref(false); // for the animation
const viewer = ref<HTMLElement | null>(null);

let book: Book | null = null;
let rendition: Rendition | null = null;
let epubHashDebounceTimer: number | null = null;
let resizeObserver: ResizeObserver | null = null;
let resizeTimer: number | null = null;
let isUnmounted = false;
let onRelocatedHandler: ((loc: unknown) => void) | null = null;
let onWindowHashChangeHandler: (() => void) | null = null;

function applyTheme() {
  if (!rendition) return;
  const rootStyle = getComputedStyle(document.documentElement);
  const background = rootStyle.getPropertyValue("--background").trim();
  const text = rootStyle.getPropertyValue("--textPrimary").trim();
  const link = rootStyle.getPropertyValue("--primaryColor").trim();
  rendition.themes.default({
    "html, body": { background: `${background} !important`, color: `${text} !important` },
    a: { color: `${link} !important` },
    "p, h1, h2, h3, h4, h5, h6, li": { color: `${text} !important` },
  });
}

function nextPage() {
  void rendition?.next();
}

function prevPage() {
  void rendition?.prev();
}

function onLoadComponentError(error: unknown) {
  console.error("Error loading EPUB file:", error);
}

watch(() => getters.isDarkMode(), () => { applyTheme() }, { flush: "post" });

onMounted(async () => {
  mutations.resetSelected();
  mutations.addSelected({
    name: state.req.name ?? "",
    path: state.req.path ?? "",
    size: state.req.size,
    type: state.req.type,
    source: state.req.source,
    modified: state.req.modified,
    hasPreview: state.req.hasPreview,
  });
  try {
    const viewIdentity = requestViewIdentity(state.req);
    let viewToken: string | undefined =
      getRequestViewToken(state.req) ?? getCachedViewToken(state.req.source ?? "");
    try {
      const refreshed = await ensureViewToken(state.req.source ?? "");
      if (refreshed) {
        viewToken = refreshed;
        if (requestViewIdentity(state.req) === viewIdentity) {
          mutations.setRequestViewToken(refreshed);
        }
      }
    } catch (err) {
      console.warn("Failed to refresh view token for EPUB preview:", err);
    }

    const epubUrl = getters.isShare()
      ? resourcesApi.getViewURL(
          state.req.source,
          state.req.path,
          viewToken,
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
          },
          false,
          state.req.type || state.req.name,
        )
      : resourcesApi.getViewURL(
          state.req.source,
          state.req.path,
          viewToken,
          null,
          false,
          state.req.type || state.req.name,
        );

    // Initialize the EPUB book (async)
    const { default: ePub } = await import("@likecoin/epub-ts");
    if (isUnmounted) return;
    const newBook = ePub(epubUrl, { openAs: "epub" });
    book = newBook;

    // Render the book to the viewer div
    const newRendition = newBook.renderTo("viewer", {
      width: "100%",
      height: "100%",
      spread: "auto",
      flow: "paginated",
    });
    rendition = newRendition;

    let manager: object | undefined;
    Object.defineProperty(newRendition, "manager", {
      configurable: true,
      get: () => manager,
      set: (value?: object) => {
        manager = value;
        if (!value) return;
        let stage: { size: (width?: string | number | null, height?: string | number | null) => unknown } | undefined;
        Object.defineProperty(value, "stage", {
          configurable: true,
          get: () => stage,
          set: (created: typeof stage) => {
            if (created) {
              const size = created.size.bind(created);
              created.size = (width, height) => size(width ?? "100%", height ?? "100%");
            }
            stage = created;
          },
        });
      },
    });

    // restore from `#epubcfi=...` if present, else first linear chapter
    const initialCfi = parseEpubCfiFromHash();
    try {
      if (initialCfi) {
        await newRendition.display(initialCfi);
      } else {
        await newRendition.display();
      }
    } catch {
      await newRendition.display();
    }
    if (isUnmounted) return;

    applyTheme();

    onRelocatedHandler = (loc: unknown) => {
      const start = (loc as { start?: { cfi?: unknown } })?.start;
      const cfi = cfiToString(start?.cfi);
      if (!cfi) return;
      if (epubHashDebounceTimer !== null) {
        clearTimeout(epubHashDebounceTimer);
      }
      epubHashDebounceTimer = window.setTimeout(() => {
        epubHashDebounceTimer = null;
        replaceUrlHashWithEpubCfi(cfi);
      }, 300);
    };
    newRendition.on("relocated", onRelocatedHandler);

    onWindowHashChangeHandler = () => {
      const cfi = parseEpubCfiFromHash();
      if (!cfi) return;
      newRendition.display(cfi).catch(() => {});
    };
    window.addEventListener("hashchange", onWindowHashChangeHandler);

    const viewerEl = viewer.value;
    if (viewerEl) {
      resizeObserver = new ResizeObserver(() => {
        if (resizeTimer !== null) {
          clearTimeout(resizeTimer);
        }
        resizeTimer = window.setTimeout(() => {
          resizeTimer = null;
          rendition?.resize(viewerEl.clientWidth, viewerEl.clientHeight);
        }, 100);
      });
      resizeObserver.observe(viewerEl);
    }

    // flags to show the book and trigger animation
    isReady.value = true;
    setTimeout(() => {
      floatIn.value = true;
    }, 100); // slight delay to allow rendering
  } catch (error) {
    onLoadComponentError(error);
  }
});

onBeforeUnmount(() => {
  isUnmounted = true;
  resizeObserver?.disconnect();
  resizeObserver = null;
  if (resizeTimer !== null) {
    clearTimeout(resizeTimer);
    resizeTimer = null;
  }
  if (epubHashDebounceTimer !== null) {
    clearTimeout(epubHashDebounceTimer);
    epubHashDebounceTimer = null;
  }
  if (onWindowHashChangeHandler) {
    window.removeEventListener("hashchange", onWindowHashChangeHandler);
    onWindowHashChangeHandler = null;
  }
  if (rendition && onRelocatedHandler) {
    rendition.off("relocated", onRelocatedHandler);
    onRelocatedHandler = null;
  }
  if (book) {
    book.destroy();
  }
});
</script>

<style scoped>
.epub-container {
  position: relative;
  width: 100%;
  height: 100%;
  background-color: var(--background); /* background for the reader */
  z-index: 1000;
}

.loading-indicator {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  font-size: 1.2em;
  color: var(--textSecondary);
}

/* The viewer must have a defined height for epub.js to work */
#viewer {
  width: 100%;
  height: 100%;
  visibility: hidden; /* Hide until ready to prevent flicker */
}

#viewer.ready {
  visibility: visible;
}

.navigation {
  position: absolute;
  bottom: 1em;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1001; /* Ensure controls are on top */
  display: flex;
  gap: 1em;
  background-color: var(--surfacePrimary);
  padding: 0.5em;
  border-radius: var(--borderRadius);
  box-shadow: 0 2px 10px rgb(0 0 0 / 10%);
  align-items: center;
}

.nav-button {
  background-color: transparent;
  border-radius: var(--borderRadius);
  font-size: 1.5em;
  color: var(--textPrimary);
  cursor: pointer;
  padding: 0.25em 1em;
  transition: background-color 0.2s;
}

.nav-button:hover {
  background-color: var(--hoverOverlay);
  color: var(--primaryColor);
}
</style>
