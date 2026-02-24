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
import { defineComponent } from "vue";
import ePub, { type Book, type Rendition } from "epubjs";
import { state, mutations, getters } from "@/store"; // Assuming your store setup
import { resourcesApi, publicApi } from "@/api"; // Assuming your api setup
import { removeLastDir } from "@/utils/url"; // Assuming your utils setup

export default defineComponent({
  name: "epubViewer",
  data() {
    return {
      isReady: false, // Flag to indicate when the book is loaded
      floatIn: false, // Flag for the float-in animation
      book: null as Book | null,
      rendition: null as Rendition | null,
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
        ? publicApi.getDownloadURL({
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

      // 4. Display the rendered book
      await this.rendition.display();

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
    // Clean up and destroy the epub instance to free up memory
    if (this.book) {
      this.book.destroy();
    }
  },
  methods: {
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
  background-color: var(--surfaceSecondary); /* A light background for the reader */
  z-index: 1000;
}

.loading-indicator {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  font-size: 1.2em;
  color: #6c757d;
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
  background-color: rgba(255, 255, 255, 0.8);
  padding: 0.5em;
  border-radius: 8px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
}

.nav-button {
  background-color: transparent;
  border: none;
  font-size: 1.5em;
  color: #343a40;
  cursor: pointer;
  padding: 0.25em 1em;
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
}
</style>
