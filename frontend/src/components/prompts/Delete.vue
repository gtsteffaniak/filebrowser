<template>
    <div class="card-content">
      <!-- Loading spinner overlay -->
      <div v-show="deleting" class="loading-content">
        <LoadingSpinner size="small" />
        <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
      </div>
      <div v-show="!deleting">
        <p v-if="itemsToDelete.length === 1">
          {{ $t("prompts.deleteMessageSingle") }}
        </p>
        <p v-else>
          {{ $t("prompts.deleteMessageMultiple", { count: itemsToDelete.length }) }}
        </p>
        <div class="delete-items-list">
        <div
          v-for="(item, index) in itemsToDelete"
          :key="index"
          class="delete-item-wrapper"
          :class="{ 'has-error': getItemError(item) }"
        >
          <ListingItem
            :name="getItemName(item.path)"
            :isDir="item.type === 'directory'"
            :source="item.source"
            :type="item.type"
            :size="item.size"
            :modified="item.modified"
            :index="index"
            :path="item.path"
            :hasPreview="item.hasPreview"
            :displayFullPath="true"
            :updateGlobalState="false"
            :isSelectedProp="false"
            :clickable="false"
            @click="preventInteraction"
            @select="preventInteraction"
            class="delete-listing-item"
          />
          <div v-if="getItemError(item)" class="error-banner">
            {{ getItemError(item) }}
          </div>
        </div>
      </div>
      </div>
    </div>
    <div class="card-actions">
      <button @click="closeHovers" class="button button--flat button--grey" :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button @click="submit" class="button button--flat button--red" aria-label="Confirm-Delete"
        :title="$t('general.delete')">
        {{ $t("general.delete") }}
      </button>
    </div>
</template>

<script>
import { filesApi, publicApi } from "@/api";
import buttons from "@/utils/buttons";
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";
import { getTypeInfo } from "@/utils/mimetype";
import ListingItem from "@/components/files/ListingItem.vue";
import { eventBus } from "@/store/eventBus";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "delete",
  components: {
    ListingItem,
    LoadingSpinner,
  },
  props: {
    items: {
      type: Array,
      default: null, // If not provided, will compute from state
    },
  },
  data() {
    return {
      failedItems: [], // Array of failed items with source, path, and message
      deleting: false,
    };
  },
  mounted() {
    const count = this.itemsToDelete.length;
    if (count > 0) {
      if (state.user.deleteWithoutConfirming && count === 1) {
        this.submit();
      }
    }
  },
  computed: {
    isListing() {
      return getters.isListing();
    },
    itemsToDelete() {
      // If props.items is provided, use it
      if (this.items && Array.isArray(this.items) && this.items.length > 0) {
        return this.items;
      }

      // Otherwise, compute from state (backward compatibility)
      let items = [];

      if (state.isSearchActive || getters.currentView() == "preview") {
        const selected = state.selected[0];
        const item = state.req.items?.[selected] || selected;
        const previewUrl = this.getPreviewUrl(item.source || state.req.source, item.path, item.modified, item.type);
        items.push({
          source: item.source || state.req.source,
          path: item.path,
          type: item.type,
          size: item.size,
          modified: item.modified,
          previewUrl: previewUrl,
          hasPreview: item.hasPreview,
        });
      } else if (!this.isListing) {
        const item = state.req.items[state.selected[0]];
        const previewUrl = this.getPreviewUrl(item.source || state.req.source, item.path, item.modified, item.type);
        items.push({
          source: item.source || state.req.source,
          path: item.path,
          type: item.type,
          size: item.size,
          modified: item.modified,
          previewUrl: previewUrl,
          hasPreview: item.hasPreview,
        });
      } else {
        for (let index of state.selected) {
          const item = state.req.items[index];
          const previewUrl = this.getPreviewUrl(item.source || state.req.source, item.path, item.modified, item.type);
          items.push({
            source: item.source || state.req.source,
            path: item.path,
            type: item.type,
            size: item.size,
            modified: item.modified,
            previewUrl: previewUrl,
            hasPreview: item.hasPreview,
          });
        }
      }

      return items;
    },
  },

  methods: {
    preventInteraction(event) {
      // Handle both DOM events and custom event objects
      if (event && typeof event.preventDefault === 'function') {
        event.preventDefault();
      }
      if (event && typeof event.stopPropagation === 'function') {
        event.stopPropagation();
      }
      // For custom events (like @select), just return early
      return false;
    },
    closeHovers() {
      mutations.closeHovers();
    },
    getItemName(path) {
      const parts = path.split("/").filter(p => p);
      return parts[parts.length - 1] || path;
    },
    getItemError(item) {
      // Find matching failed item by comparing source and path
      for (const failedItem of this.failedItems.values()) {
        if (failedItem.source === item.source && failedItem.path === item.path) {
          return failedItem.message || 'Unknown error';
        }
      }
      return null;
    },
    getPreviewUrl(source, path, modified, type) {
      if (!source || !path) return null;

      // Check if file type supports previews
      const typeInfo = getTypeInfo(type || '');
      const simpleType = typeInfo.simpleType;

      if (simpleType === 'directory') return null;
      if (simpleType !== 'image' && simpleType !== 'video' && simpleType !== 'document' && simpleType !== 'text') {
        return null;
      }

      // Check preview permissions
      if (simpleType === 'video' && !getters.previewPerms().video) return null;
      if (simpleType === 'image' && !getters.previewPerms().image) return null;
      if ((simpleType === 'document' || simpleType === 'text') && !getters.previewPerms().office) return null;

      try {
        return filesApi.getPreviewURL(source, path, modified);
      } catch (e) {
        return null;
      }
    },
    async submit() {
      if (this.deleting) {
        return;
      }

      this.deleting = true;
      this.failedItems = [];
      buttons.loading("delete");

      try {
        // Extract source and path from items (ignore previewUrl)
        const itemsForDelete = this.itemsToDelete.map(item => ({
          source: item.source,
          path: item.path
        }));

        if (itemsForDelete.length === 0) {
          buttons.done("delete");
          this.deleting = false;
          return;
        }

        // Use bulk delete API for both regular files and shares
        const response = getters.isShare()
          ? await publicApi.bulkDelete(itemsForDelete)
          : await filesApi.bulkDelete(itemsForDelete);

        // Store failed items directly from response
        if (response.failed && response.failed.length > 0) {
          this.failedItems = response.failed;
        } else {
          this.failedItems = [];
        }

        const succeededCount = response.succeeded ? response.succeeded.length : 0;
        const failedCount = response.failed ? response.failed.length : 0;

        if (failedCount === 0) {
          // All succeeded - close prompt and reload
          buttons.success("delete");
          notify.showSuccessToast(this.$t('prompts.deleted'));

          if (this.items && this.items.length > 0) {
            eventBus.emit("itemsDeleted", {
              succeeded: response.succeeded || [],
              failed: []
            });
          }

          mutations.closeHovers();
          if (!this.items) {
            mutations.resetSelected();
          }
          mutations.setReload(true);
        } else if (succeededCount > 0) {
          // Partial success - keep prompt open to show errors
          buttons.done("delete");

          // Emit event with partial results if items were passed as props
          if (this.items && this.items.length > 0) {
            eventBus.emit("itemsDeleted", {
              succeeded: response.succeeded || [],
              failed: response.failed || []
            });
          }
        } else {
          // All failed
          buttons.done("delete");
        }

        this.deleting = false;
      } catch (e) {
        buttons.done("delete");
        this.deleting = false;
        console.error(e);
        // On network/API errors, show error for all items
        this.failedItems = this.itemsToDelete.map(item => ({
          source: item.source,
          path: item.path,
          message: e.message || 'Delete failed'
        }));
      }
    },
  },
};
</script>

<style scoped>
.delete-items-list {
  max-height: 400px;
  overflow-y: auto;
  margin-top: 1rem;
  padding: 0.5rem;
}

.delete-item-wrapper {
  margin-bottom: 0.5rem;
}

.delete-item-wrapper:last-child {
  margin-bottom: 0;
}

.delete-item-wrapper.has-error {
  border-left: 3px solid var(--errorColor, #f44336);
  padding-left: 0.5rem;
}

.delete-listing-item a {
  pointer-events: none;
  cursor: default !important;
  padding: 0.25em;
}

.delete-listing-item.listing-item {
  cursor: default !important;
}

.error-banner {
  margin-top: 0.5rem;
  padding: 0.5rem;
  background: var(--errorBackground, rgba(244, 67, 54, 0.1));
  color: var(--errorColor, #f44336);
  border-radius: 4px;
  font-size: 0.875rem;
}

.loading-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding-top: 2em;
}

.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}

.card-content {
  position: relative;
}
</style>
