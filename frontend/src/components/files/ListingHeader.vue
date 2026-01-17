<template>
  <div class="listing-header-wrapper">
    <div class="header card" :class="{ 'dark-mode-item-header': isDarkMode }">
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
        <i class="material-icons">{{ nameIcon }}</i>
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
        <i class="material-icons">{{ sizeIcon }}</i>
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
        <i class="material-icons">{{ modifiedIcon }}</i>
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
        <i class="material-icons">{{ durationIcon }}</i>
      </p>
    </div>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";

export default {
  name: "ListingHeader",
  props: {
    hasDuration: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
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
.listing-header-wrapper {
  margin-bottom: 0;
  width: 100%;
  box-sizing: border-box;
  padding-right: 0.5em;
}

.header {
  display: flex !important;
  background: white;
  border: 1px solid rgba(0, 0, 0, .1);
  z-index: 999;
  padding: .85em;
  width: 100%;
  box-sizing: border-box;
  border-top-left-radius: 1em;
  border-top-right-radius: 1em;
  border: unset;
  margin-bottom: 0.35em;
}

.dark-mode-item-header {
  border-color: var(--divider) !important;
  background: var(--surfacePrimary) !important;
  user-select: none;
}

.header .name {
  margin-right: 1.5em;
  width: 50%;
  box-sizing: border-box;
}

.header>p {
  margin: 0;
  cursor: pointer;
  box-sizing: border-box;
}

.header .size {
  width: 25%;
  box-sizing: border-box;
}

.header .duration {
  margin-left: auto;
  padding-right: 1em;
  box-sizing: border-box;
}

.header i {
  font-size: 1.5em;
  vertical-align: middle;
  margin-left: .2em;
  opacity: 0;
  transition: .1s ease all;
}

.header p:hover i,
.header .active i {
  opacity: 1;
}

.header .active {
  font-weight: bold;
}

.header span {
  vertical-align: middle;
}
</style>
