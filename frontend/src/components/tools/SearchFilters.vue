<template>
  <SettingsItem
    :title="expanded ? $t('buttons.showLess') : $t('buttons.showMore')"
    :collapsable="true"
    :start-collapsed="!expanded"
    @toggle="expanded = $event"
  >
    <div class="search-filters-inner">
      <div class="search-filter-dropdowns">
        <ExpandDropdown
          v-model="filters.entryTypeFilter"
          :options="entryTypeOptions"
          all-value="all"
          :all-selected-label="$t('search.filesAndFolders')"
          :aria-label="$t('search.filesAndFolders')"
        />
        <ExpandDropdown
          v-model="filters.selectedMediaTypes"
          :options="mediaTypeOptions"
          allow-multiple
          empty-means-all
          :all-selected-label="$t('search.allFileTypes')"
          :disabled="foldersOnly"
          :aria-label="$t('search.allFileTypes')"
        />
      </div>
      <div class="constraints">
        <div class="sizeInputWrapper">
          <p>{{ $t("search.smallerThan") }}</p>
          <input
            class="sizeInput input"
            v-model="filters.smallerThan"
            type="number"
            min="0"
            placeholder="MB"
          />
          <p>{{ $t("search.largerThan") }}</p>
          <input class="sizeInput input" v-model="filters.largerThan" type="number" placeholder="MB" />
        </div>
        <div class="sizeInputWrapper">
          <p>{{ $t("search.olderThanDate") }}</p>
          <input class="sizeInput input" v-model="filters.modifiedOlderThan" type="date" />
          <p>{{ $t("search.newerThanDate") }}</p>
          <input class="sizeInput input" v-model="filters.modifiedNewerThan" type="date" />
        </div>
      </div>
      <div class="settings-items">
        <slot name="extra-settings" />
        <ToggleSwitch
          class="item"
          v-model="filters.useWildcardSearch"
          :name="$t('search.useWildcardSearch')"
          :description="$t('search.useWildcardSearchDescription')"
        />
        <ToggleSwitch
          class="item"
          v-model="filters.caseExactSearch"
          :name="$t('search.caseExact')"
          :description="$t('search.caseExactDescription')"
        />
      </div>
    </div>
  </SettingsItem>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import type { SearchFilters } from "@/composables/useSearchFilters";

// Whether the options are open
const expanded = defineModel<boolean>("expanded", { default: false });
const props = defineProps<{ filters: SearchFilters }>();
defineOptions({ name: "search-filters" });
const filters = props.filters;

const { t } = useI18n();

const foldersOnly = computed(() => filters.entryTypeFilter === "type:folder");
const entryTypeOptions = computed(() => [
  { value: "all", label: t("search.filesAndFolders") },
  { value: "type:file", label: t("search.onlyFiles") },
  { value: "type:folder", label: t("search.onlyFolders") },
]);
const mediaTypeOptions = computed(() => [
  { value: "type:image", label: t("general.photos") },
  { value: "type:audio", label: t("general.audio") },
  { value: "type:video", label: t("general.videos") },
  { value: "type:doc", label: t("general.documents") },
  { value: "type:archive", label: t("general.archives") },
]);
</script>

<style scoped>
.search-filters-inner {
  box-sizing: border-box;
  width: 100%;
}

.search-filter-dropdowns {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5em;
  align-items: flex-start;
  justify-content: center;
  margin-bottom: 0.5em;
}

.search-filter-dropdowns :deep(.expand-dropdown) {
  width: auto;
  flex: 0 1 auto;
  min-width: 15em;
  max-width: 15em;
}

.search-filter-dropdowns :deep(.expand-dropdown-anchor.menu-panel) {
  min-width: 0;
}
</style>
