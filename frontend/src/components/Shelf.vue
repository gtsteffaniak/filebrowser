<template>
  <transition name="shelf-slide">
    <div
      v-show="!isHidden && showShelf"
      id="shelf"
      :class="{ 'add-padding': addPadding }"
      :style="moveWithSidebar"
    >
      <Breadcrumbs v-if="showBreadcrumbs" />
      <ListingHeader v-if="showListingHeader" :hasDuration="hasDuration" />
      <DuplicateFinderActions
        v-if="showDuplicateFinderActions"
        :selectedCount="duplicateFinderSelectedCount"
        :deleting="duplicateFinderDeleting"
        @delete="handleDuplicateFinderDelete"
        @clear="handleDuplicateFinderClear"
      />
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import Breadcrumbs from "@/components/files/Breadcrumbs.vue";
import ListingHeader from "@/components/files/ListingHeader.vue";
import DuplicateFinderActions from "@/components/tools/DuplicateFinderActions.vue";
import { getters, state } from "@/store/index.ts";
import { eventBus } from "@/store/eventBus.ts";

const route = useRoute();

const isHidden = ref(false);
const duplicateFinderSelectedCount = ref(0);
const duplicateFinderDeleting = ref(false);

let lastScrollTop = 0;
let scrollElement: Element | null = null;
let scrollFrame: number | null = null;

const addPadding = computed(() => getters.isStickySidebar() || getters.isShare());
const showBreadcrumbs = computed(() => getters.showBreadCrumbs());
const showListingHeader = computed(() => {
  if ((state.route.path || "") === "/tools/advancedSearch") return false;
  if ((state.req?.items?.length ?? 0) <= 0) return false;
  return getters.currentView() === "listingView";
});

// Show duplicate finder actions when on that route and there are selected items
const showDuplicateFinderActions = computed(() => duplicateFinderSelectedCount.value > 0);

const showShelf = computed(
  () => showBreadcrumbs.value || showListingHeader.value || showDuplicateFinderActions.value,
);

// Whether any file has duration metadata (scanned once per listing, shared with ListingView)
const hasDuration = computed(() => getters.hasDuration());

const moveWithSidebar = computed(() =>
  getters.isStickySidebar() && getters.isSidebarVisible() ? { left: "var(--sidebar-width)" } : {},
);

function handleScroll(event: Event) {
  // Use requestAnimationFrame to throttle updates (like Scrollbar component)
  if (scrollFrame) return;
  const target = event.target as HTMLElement;

  scrollFrame = requestAnimationFrame(() => {
    scrollFrame = null;
    const scrollTop = target.scrollTop;

    // Always show when at the top
    if (scrollTop <= 10) {
      isHidden.value = false;
      lastScrollTop = scrollTop;
      return;
    }
    // Only trigger if scrolled enough (at least 30px)
    const scrollDiff = scrollTop - lastScrollTop;
    if (Math.abs(scrollDiff) < 30) return;
    // Hide when scrolling down, show when scrolling up
    isHidden.value = scrollDiff > 0;
    lastScrollTop = scrollTop;
  });
}

function attachScrollListener() {
  // Try to find the scroll wrapper (used by Scrollbar component)
  const scrollWrapper = document.querySelector(".scroll-wrapper");
  if (scrollWrapper) {
    scrollWrapper.addEventListener("scroll", handleScroll, { passive: true });
    scrollElement = scrollWrapper;
  }
}

function detachScrollListener() {
  if (scrollElement) {
    scrollElement.removeEventListener("scroll", handleScroll);
    scrollElement = null;
  }
  // Cancel any pending animation frame
  if (scrollFrame) {
    cancelAnimationFrame(scrollFrame);
    scrollFrame = null;
  }
}

// Duplicate finder event handlers
function handleDuplicateFinderSelectionChanged(count: number) {
  duplicateFinderSelectedCount.value = count;
}
function handleDuplicateFinderDeletingChanged(deleting: boolean) {
  duplicateFinderDeleting.value = deleting;
}
function handleDuplicateFinderDelete() {
  eventBus.emit("duplicateFinderDeleteRequested");
}
function handleDuplicateFinderClear() {
  eventBus.emit("duplicateFinderClearRequested");
}

watch(
  () => route.fullPath,
  () => {
    isHidden.value = false;
    lastScrollTop = 0;
  },
);

watch(showShelf, (visible, wasVisible) => {
  if (visible && !wasVisible) isHidden.value = false;
});

onMounted(async () => {
  eventBus.on<number>("duplicateFinderSelectionChanged", handleDuplicateFinderSelectionChanged);
  eventBus.on<boolean>("duplicateFinderDeletingChanged", handleDuplicateFinderDeletingChanged);
  await nextTick();
  attachScrollListener();
});

onBeforeUnmount(() => {
  detachScrollListener();
  eventBus.off<number>("duplicateFinderSelectionChanged", handleDuplicateFinderSelectionChanged);
  eventBus.off<boolean>("duplicateFinderDeletingChanged", handleDuplicateFinderDeletingChanged);
});
</script>

<style scoped>
#shelf {
  overflow: hidden visible;
  position: fixed;
  padding: 0.5em 0.75em 0;
  z-index: 1000;
  right: 0;
  left: 0;
  transition: left 0.22s cubic-bezier(0.2, 0, 0, 1);
  box-sizing: border-box;
  height: auto;
  min-height: 0;
  pointer-events: auto;
  backdrop-filter: blur(5px);
}

.shelf-slide-enter-active,
.shelf-slide-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease !important;
}

.shelf-slide-enter-from,
.shelf-slide-leave-to {
  transform: translateY(-100%);
  opacity: 0;
}

.shelf-slide-enter-to,
.shelf-slide-leave-from {
  transform: translateY(0);
  opacity: 1;
}

</style>
