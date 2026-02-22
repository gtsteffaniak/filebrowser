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
      <i v-if="nameSorted" class="material-icons">{{ nameIcon }}</i>
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
      <span>{{ $t("general.size") }}</span>
      <i v-if="sizeSorted" class="material-icons">{{ sizeIcon }}</i>
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
      <span>{{ $t("files.lastModified") }}</span>
      <i v-if="modifiedSorted" class="material-icons">{{ modifiedIcon }}</i>
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
      <span>{{ $t("files.duration") }}</span>
      <i v-if="durationSorted" class="material-icons">{{ durationIcon }}</i>
    </p>
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

.name {
  margin-right: 1.5em;
  box-sizing: border-box;
}

p {
  margin: 0;
  cursor: pointer;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  min-width: 0;
}

span {
  vertical-align: middle;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.size {
  box-sizing: border-box;
}

.duration {
  margin-left: auto;
  padding-right: 1em;
  box-sizing: border-box;
}

.desktop-view {
  justify-content: unset !important;
}

/* Desktop-specific column widths */
.desktop-view .name {
  width: 50%;
}

.desktop-view .size {
  width: 25%;
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
