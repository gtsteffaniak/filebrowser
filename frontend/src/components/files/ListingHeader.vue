<template>
  <div class="listing-item-header card" :class="{ 'dark-mode': isDarkMode, 'desktop-view': !isMobile }">
    <p
      :class="{ active: nameSorted }"
      class="name"
      role="button"
      tabindex="0"
      @click="sort('name')"
      :title="$t('files.sortByName')"
      :aria-label="$t('files.sortByName')"
    >
      <span>{{ $t("general.name") }}</span>
      <i v-if="nameSorted" class="material-symbols">{{ nameIcon }}</i>
    </p>

    <p
      :class="{ active: sizeSorted }"
      class="size"
      role="button"
      tabindex="0"
      @click="sort('size')"
      :title="$t('files.sortBySize')"
      :aria-label="$t('files.sortBySize')"
    >
      <i v-if="sizeSorted" class="material-symbols">{{ sizeIcon }}</i>
      <span>{{ $t("general.size") }}</span>
    </p>

    <p
      :class="{ active: modifiedSorted }"
      class="modified"
      role="button"
      tabindex="0"
      @click="sort('modified')"
      :title="$t('files.sortByLastModified')"
      :aria-label="$t('files.sortByLastModified')"
    >
      <i v-if="modifiedSorted" class="material-symbols">{{ modifiedIcon }}</i>
      <span>{{ $t("files.lastModified") }}</span>
    </p>

    <p
      v-if="hasDuration"
      :class="{ active: durationSorted }"
      class="duration"
      role="button"
      tabindex="0"
      @click="sort('duration')"
      :title="$t('files.sortByDuration')"
      :aria-label="$t('files.sortByDuration')"
    >
      <i v-if="durationSorted" class="material-symbols">{{ durationIcon }}</i>
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
  },
  computed: {
    isMobile() {
      return state.isMobile;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    nameSorted() {
      return getters.sorting().by === "name";
    },
    sizeSorted() {
      return getters.sorting().by === "size";
    },
    modifiedSorted() {
      return getters.sorting().by === "modified";
    },
    durationSorted() {
      return getters.sorting().by === "duration";
    },
    ascOrdered() {
      return getters.sorting().asc;
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
    durationIcon() {
      if (this.durationSorted && this.ascOrdered) {
        return "arrow_downward";
      }
      return "arrow_upward";
    },
    quickDownloadEnabled() {
      // @ts-ignore
      if (state.isMobile) {
        return false
      }
      if (getters.isShare()) {
        // @ts-ignore
        return state.shareInfo?.quickDownload;
      }
      // @ts-ignore
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
        (field === "duration" && this.durationIcon === "arrow_upward")
      ) {
        asc = true;
      }
      // Commit the updateSort mutation
      mutations.updateListingSortConfig({ field, asc });
      mutations.updateListingItems();
    },
  },
};
</script>

<style scoped>
.listing-item-header {
  display: flex;
  background: white;
  border: 1px solid rgba(0, 0, 0, .1);
  z-index: 999;
  padding: .85em;
  width: 100%;
  box-sizing: border-box;
  border-top-left-radius: 1em;
  border-top-right-radius: 1em;
  border: unset;
  margin-bottom: 0 !important;
  justify-content: space-between;
}

.dark-mode {
  border-color: var(--divider) !important;
  background: var(--surfacePrimary) !important;
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
}

.desktop-view .modified{
  min-width: 15%;
  flex: 0;
}

.desktop-view .size,
.desktop-view .duration {
  min-width: 10%;
  flex: 0;
}

.size,
.modified,
.duration {
  flex: 1;
  justify-content: flex-end;
  text-align: end;
}


i {
  font-size: 1.5em;
  vertical-align: middle;
  margin-left: .2em;
  opacity: 0;
  transition: .1s ease all;
  flex-shrink: 0;
}

p:hover i,
.active i {
  opacity: 1;
}

.active {
  font-weight: bold;
}
</style>
