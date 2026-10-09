<template>
  <div class="listing-item-header card" :class="{ 'desktop-view': !isMobile }">
    <p
      :class="{ active: nameSorted }"
      class="name"
      role="button"
      tabindex="0"
      @click="sort('name')"
      @keydown.enter.prevent="sort('name')"
      @keydown.space.prevent="sort('name')"
      :title="$t('files.sortByName')"
      :aria-label="$t('files.sortByName')"
    >
      <span>{{ $t("general.name") }}</span>
      <i class="material-symbols">{{ nameIcon }}</i>
    </p>

    <p
      :class="{ active: sizeSorted }"
      class="size"
      role="button"
      tabindex="0"
      @click="sort('size')"
      @keydown.enter.prevent="sort('size')"
      @keydown.space.prevent="sort('size')"
      :title="$t('files.sortBySize')"
      :aria-label="$t('files.sortBySize')"
    >
      <i class="material-symbols">{{ sizeIcon }}</i>
      <span>{{ $t("general.size") }}</span>
    </p>

    <p
      :class="{ active: modifiedSorted }"
      class="modified"
      role="button"
      tabindex="0"
      @click="sort('modified')"
      @keydown.enter.prevent="sort('modified')"
      @keydown.space.prevent="sort('modified')"
      :title="$t('files.sortByLastModified')"
      :aria-label="$t('files.sortByLastModified')"
    >
      <i class="material-symbols">{{ modifiedIcon }}</i>
      <span>{{ $t("files.lastModified") }}</span>
    </p>

    <p
      v-if="showKindColumn"
      :class="{ active: kindSorted }"
      class="kind"
      role="button"
      tabindex="0"
      @click="sort('kind')"
      @keydown.enter.prevent.stop="sort('kind')"
      @keydown.space.prevent.stop="sort('kind')"
      :title="$t('files.sortByType')"
      :aria-label="$t('files.sortByType')"
    >
      <span>{{ $t("general.type") }}</span>
      <i class="material-symbols">{{ kindIcon }}</i>
    </p>

    <p
      v-if="showCreatedColumn"
      :class="{ active: createdSorted }"
      class="created"
      role="button"
      tabindex="0"
      @click="sort('created')"
      @keydown.enter.prevent.stop="sort('created')"
      @keydown.space.prevent.stop="sort('created')"
      :title="$t('files.sortByCreationTime')"
      :aria-label="$t('files.sortByCreationTime')"
    >
      <i class="material-symbols">{{ createdIcon }}</i>
      <span>{{ $t("files.creationTime") }}</span>
    </p>

    <p
      v-if="hasDuration"
      :class="{ active: durationSorted }"
      class="duration"
      role="button"
      tabindex="0"
      @click="sort('duration')"
      @keydown.enter.prevent="sort('duration')"
      @keydown.space.prevent="sort('duration')"
      :title="$t('files.sortByDuration')"
      :aria-label="$t('files.sortByDuration')"
    >
      <i class="material-symbols">{{ durationIcon }}</i>
      <span>{{ $t("files.duration") }}</span>
    </p>
    <span v-if="quickDownloadEnabled" class="placeholder"></span>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";

export default {
  name: "ListingHeader",
  props: {
    hasDuration: {
      type: Boolean,
      default: false,
    },
    /** When true, sort via pickerSorting (destination pickers) instead of the main listing sort. */
    usePickerSorting: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
    isMobile() {
      return getters.isMobile();
    },
    sortConfig() {
      return this.usePickerSorting ? getters.pickerSorting() : getters.sorting();
    },
    nameSorted() {
      return this.sortConfig.by === "name";
    },
    sizeSorted() {
      return this.sortConfig.by === "size";
    },
    modifiedSorted() {
      return this.sortConfig.by === "modified";
    },
    createdSorted() {
      return this.sortConfig.by === "created";
    },
    kindSorted() {
      return this.sortConfig.by === "kind";
    },
    durationSorted() {
      return this.sortConfig.by === "duration";
    },
    ascOrdered() {
      return this.sortConfig.asc;
    },
    galleryView() {
      return getters.viewMode() === "gallery";
    },
    isListMode() {
      const mode = getters.viewMode();
      return mode === "list" || mode === "compact";
    },
    showKindColumn() {
      return this.isListMode && !this.usePickerSorting && state.user?.showTypeColumn;
    },
    showCreatedColumn() {
      return this.isListMode && !this.usePickerSorting && state.user?.showCreationDateColumn;
    },
    nameIcon() {
      if (this.nameSorted && !this.ascOrdered) {
        return "arrow_upward";
      }
      return "arrow_downward";
    },
    sizeIcon() {
      if (this.sizeSorted && this.ascOrdered) {
        return "arrow_downward";
      }
      return "arrow_upward";
    },
    modifiedIcon() {
      if (this.modifiedSorted && this.ascOrdered) {
        return "arrow_downward";
      }
      return "arrow_upward";
    },
    createdIcon() {
      if (this.createdSorted && this.ascOrdered) {
        return "arrow_downward";
      }
      return "arrow_upward";
    },
    kindIcon() {
      if (this.kindSorted && this.ascOrdered) {
        return "arrow_downward";
      }
      return "arrow_upward";
    },
    durationIcon() {
      if (this.durationSorted && this.ascOrdered) {
        return "arrow_downward";
      }
      return "arrow_upward";
    },
    quickDownloadEnabled() {
      if (this.usePickerSorting) {
        return false;
      }
      if (getters.isMobile()) {
        return false
      }
      if (getters.isShare()) {
        return state.shareInfo?.quickDownload;
      }
      return state.user?.quickDownload && !this.galleryView;
    },
  },
  methods: {
    sort(field) {
      let asc = false;
      if (
        (field === "name" && this.nameIcon === "arrow_upward") ||
        (field === "size" && this.sizeIcon === "arrow_upward") ||
        (field === "modified" && this.modifiedIcon === "arrow_upward") ||
        (field === "created" && this.createdIcon === "arrow_upward") ||
        (field === "kind" && this.kindIcon === "arrow_upward") ||
        (field === "duration" && this.durationIcon === "arrow_upward")
      ) {
        asc = true;
      }
      // Commit the updateSort mutation
      if (this.usePickerSorting) {
        mutations.updatePickerSortConfig({ field, asc });
        return;
      }
      mutations.updateListingSortConfig({ field, asc });
      mutations.updateListingItems();
    },
  },
};
</script>

<style scoped>
.listing-item-header {
  display: flex;
  background: var(--surfacePrimary);
  border: 1px solid var(--divider);
  z-index: 999;
  padding: .85em;
  width: 100%;
  box-sizing: border-box;
  border-top-left-radius: 1em;
  border-top-right-radius: 1em;
  margin-bottom: 0 !important;
  justify-content: space-between;
  user-select: none;
}

p {
  margin: 0;
  cursor: pointer;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  width: 100%;
}

span {
  vertical-align: middle;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.name {
  flex: 1;
  min-width: 0;
}

.size,
.modified,
.duration {
  flex: 1;
  justify-content: flex-end;
  text-align: end;
}

/* Column widths are shared with the list/compact rows (see listing.css) so the
   header lines up with the items and the name column gets the remaining space. */
/* stylelint-disable no-unknown-custom-properties -- defined in css/listing.css */
.desktop-view {
  /* same right inset as the list/compact rows */
  padding-right: var(--listing-col-gap);
}

/* With quick download the rows end in a ~2.4rem download icon after the inset */
.desktop-view:has(> .placeholder) {
  padding-right: 0;
}

.desktop-view > .placeholder {
  flex: 0 0 auto;
  width: calc(var(--listing-col-gap) + 2.4rem);
}

.desktop-view .size,
.desktop-view .modified,
.desktop-view .kind,
.desktop-view .created,
.desktop-view .duration {
  flex: 0 0 auto;
  padding-left: var(--listing-col-gap);
  justify-content: flex-end;
  text-align: right;
}

.desktop-view .size {
  width: var(--listing-col-size);
}

.desktop-view .modified {
  width: var(--listing-col-modified);
}

.desktop-view .kind {
  width: var(--listing-col-kind);
  padding-left: calc(var(--listing-col-gap) + 0.5rem);
  justify-content: flex-start;
  text-align: left;
}

.desktop-view .kind span {
  flex-shrink: 0;
}

.desktop-view .created {
  width: var(--listing-col-created);
}

.desktop-view .duration {
  width: var(--listing-col-duration);
}
/* stylelint-enable no-unknown-custom-properties */

i {
  font-size: 1.2em;
  vertical-align: middle;
  margin-left: .1em;
  opacity: 0;
  transition: opacity 0.1s ease;
  flex-shrink: 0;
}

.active i,
p:hover i,
.active:hover i {
  opacity: 1;
}

.active,
p:hover {
  font-weight: bold;
}
</style>
