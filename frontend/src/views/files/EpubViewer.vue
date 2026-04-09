<template>
  <div class="epub-container">
    <div v-if="!isReady" class="loading-indicator">
      <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
    </div>

    <div id="viewer" :class="{ ready: isReady }"></div>

    <div v-if="isReady" class="navigation">
      <button @click="prevPage" class="nav-button">&lt;</button> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <button @click="nextPage" class="nav-button">&gt;</button> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, watch } from "vue";
import ePub, { type Book, type Rendition } from "epubjs";
import { state, mutations, getters } from "@/store"; // Assuming your store setup
import { resourcesApi } from "@/api";
import { removeLastDir } from "@/utils/url"; // Assuming your utils setup

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
  if (cfi != null && typeof (cfi as { toString?: () => string }).toString === "function") {
    return String((cfi as { toString: () => string }).toString());
  }
  return "";
}

export default defineComponent({
  name: "epubViewer",
  data() {
    return {
      isReady: false, // Flag to indicate when the book is loaded
      floatIn: false, // Flag for the float-in animation
      book: null as Book | null,
      rendition: null as Rendition | null,
      epubHashDebounceTimer: null as number | null,
      unwatchDarkMode: null as (() => void) | null,
      onRelocatedHandler: null as ((loc: unknown) => void) | null,
      onWindowHashChangeHandler: null as (() => void) | null,
    };
  },
  async mounted() {
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
    });
    try {
      // 1. Fetch the download URL for the EPUB file
      const epubUrl = getters.isShare() 
        ? resourcesApi.getDownloadURLPublic({
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
            token: state.shareInfo.token,
          }, [state.req.path])
        : await resourcesApi.getDownloadURL(
            state.req.source,
            state.req.path,
            false,
            false
          );

      // 2. Initialize the EPUB book
      this.book = ePub(epubUrl);

      // 3. Render the book to the "viewer" div
      this.rendition = this.book.renderTo("viewer", {
        width: "100%",
        height: "100%",
        spread: "auto", // Automatically handle single or double page spreads
        flow: "paginated", // Standard book-like pagination
      });

      // 4. Display: restore from `#epubcfi=...` if present, else first linear chapter
      const initialCfi = parseEpubCfiFromHash();
      try {
        if (initialCfi) {
          await this.rendition.display(initialCfi);
        } else {
          await this.rendition.display();
        }
      } catch {
        await this.rendition.display();
      }

      this.applyTheme(getters.isDarkMode());

      this.unwatchDarkMode = watch(() => getters.isDarkMode(), (isDark) => {
        this.applyTheme(isDark);
      });

      this.onRelocatedHandler = (loc: unknown) => {
        const start = (loc as { start?: { cfi?: unknown } })?.start;
        const cfi = cfiToString(start?.cfi);
        if (!cfi) return;
        if (this.epubHashDebounceTimer !== null) {
          clearTimeout(this.epubHashDebounceTimer);
        }
        this.epubHashDebounceTimer = window.setTimeout(() => {
          this.epubHashDebounceTimer = null;
          replaceUrlHashWithEpubCfi(cfi);
        }, 300);
      };
      this.rendition.on("relocated", this.onRelocatedHandler);

      this.onWindowHashChangeHandler = () => {
        if (!this.rendition) return;
        const cfi = parseEpubCfiFromHash();
        if (!cfi) return;
        this.rendition.display(cfi).catch(() => {});
      };
      window.addEventListener("hashchange", this.onWindowHashChangeHandler);

      // Set flags to show the book and trigger animations
      this.isReady = true;
      setTimeout(() => {
        this.floatIn = true;
      }, 100); // slight delay to allow rendering
    } catch (error) {
      this.onLoadComponentError(error);
    }
  },
  beforeUnmount() {
    if (this.epubHashDebounceTimer !== null) {
      clearTimeout(this.epubHashDebounceTimer);
      this.epubHashDebounceTimer = null;
    }
    if (this.onWindowHashChangeHandler) {
      window.removeEventListener("hashchange", this.onWindowHashChangeHandler);
      this.onWindowHashChangeHandler = null;
    }
    if (this.rendition && this.onRelocatedHandler) {
      this.rendition.off("relocated", this.onRelocatedHandler);
      this.onRelocatedHandler = null;
    }
    this.unwatchDarkMode?.();
    this.unwatchDarkMode = null;
    if (this.book) {
      this.book.destroy();
    }
  },
  methods: {
    applyTheme(isDark: boolean) {
      if (!this.rendition) return;
      if (isDark) {
        this.rendition.themes.default({
          body: { color: "#fff !important" },
          a: { color: "#bb86fc !important" },
          p: { color: "var(--textPrimary) !important" },
          h1: { color: "var(--textPrimary) !important" },
        });
      } else {
        this.rendition.themes.default({
          body: { color: "#000 !important" },
          a: { color: "#6200ee !important" },
        });
      }
    },
    // Navigate to the next page
    nextPage() {
      this.rendition?.next();
    },
    // Navigate to the previous page
    prevPage() {
      this.rendition?.prev();
    },
    // Close the viewer and navigate away
    close() {
      const current = window.location.pathname;
      const newPath = removeLastDir(current);
      window.location.href = newPath + "#" + state.req.name;
    },
    // Error handler
    onLoadComponentError(error: any) {
      console.error("Error loading EPUB file:", error);
      // You could add logic here to display an error message to the user
    },
  },
});
</script>

<style scoped>
.epub-container {
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
  position: fixed;
  bottom: 1.5em;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1001; /* Ensure controls are on top */
  display: flex;
  gap: 1em;
  background-color: var(--surfaceSecondary);
  padding: 0.5em;
  border-radius: 8px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
  align-items: center;
}

.nav-button {
  background-color: transparent;
  border: none;
  font-size: 1.5em;
  color: var(--textPrimary);
  cursor: pointer;
  padding: 0.25em 1em;
  transition: background-color 0.2s;
}

.nav-button:hover {
  background-color: var(--alt-background);
}

/* Copied directly from your example */
.floating-close {
  position: fixed;
  left: 50%;
  transform: translate(-50%, -5em); /* Start offscreen */
  transition: transform 0.4s ease;
  background: var(--surfaceSecondary);
  font-size: 0.5em;
  top: 0;
  z-index: 1002;
}

.float-in {
  transform: translate(-50%, 2.75em); /* Animate to final position */
}

.floating-close i {
  font-size: 2em;
  padding-right: 1em;
  padding-left: 1em;
  color: var(--textPrimary);
}
</style>
