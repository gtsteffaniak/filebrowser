<template>
  <div class="card-content">
    <!-- Loading spinner overlay -->
    <div v-show="creating" class="loading-content">
      <LoadingSpinner size="small" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!creating">
      <p>{{ $t("prompts.newDirMessage") }}</p>
      <input aria-label="New Folder Name" class="input" type="text" @keyup.enter="submit" v-model.trim="name"
        v-focus />
    </div>
  </div>

  <div class="card-actions">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button class="button button--flat" :aria-label="$t('general.create')" :title="$t('general.create')"
      @click="submit">
      {{ $t("general.create") }}
    </button>
  </div>
</template>
<script>
import { resourcesApi } from "@/api";
import { getters, mutations, state } from "@/store"; // Import your custom store
import { url } from "@/utils";
import { notify } from "@/notify";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
export default {
  name: "new-dir",
  components: {
    LoadingSpinner,
  },
  props: {
    redirect: {
      type: Boolean,
      default: true,
    },
    base: {
      type: [String, Object, null],
      default: null,
    },
  },
  data() {
    return {
      name: "",
      creating: false,
    };
  },
  computed: {
    isFiles() {
      return getters.isFiles();
    },
    isListing() {
      return getters.isListing();
    },
    // Determine parent path and source based on prop
    parentInfo() {
      if (this.base) {
        if (typeof this.base === 'string') {
          return {
            path: this.base,
            source: state.req?.source || null,
          };
        } else if (typeof this.base === 'object' && this.base.path) {
          return {
            path: this.base.path,
            source: this.base.source || state.req?.source || null,
          };
        }
      }
      // Fallback to current path
      return {
        path: state.req?.path,
        source: state.req?.source || null,
      };
    },
  },
  methods: {
    closeHovers() {
      return mutations.closeTopHover();
    },
    async submit(event) {
      try {
        event.preventDefault();
        if (this.name === "") return;
        await this.createDirectory(false);
      } catch (error) {
        console.error(error);
      }
    },

    async createDirectory(overwrite = false) {
      this.creating = true;
      try {
        const parentPath = this.parentInfo.path;
        const source = this.parentInfo.source;
        const newPath = url.joinPath(parentPath, this.name) + "/";

        if (getters.isShare()) {
          await resourcesApi.postPublic(state.shareInfo?.hash, newPath, "", overwrite, undefined, {}, true);
          mutations.setReload(true);
          mutations.closeTopHover();
          this.creating = false;
          return;
        }
        await resourcesApi.post(source, newPath, "", overwrite, undefined, {}, true);
        mutations.setReload(true);
        mutations.closeTopHover();

        // Show success notification with "go to item" button
        const buttonAction = () => {
          url.goToItem(source, newPath, {});
        };
        const buttonProps = {
          icon: "folder",
          buttons: [{
            label: this.$t("buttons.goToItem"),
            primary: true,
            action: buttonAction
          }]
        };
        notify.showSuccess(this.$t("prompts.newDirSuccess"), buttonProps);
        this.creating = false;
      } catch (error) {
        if (error.message === "conflict") {
          // Show replace-rename prompt for file/folder conflicts
          mutations.showHover({
            name: "replace-rename",
            pinned: true,
            confirm: async (event, option) => {
              event.preventDefault();
              try {
                if (option === "overwrite") {
                  await this.createDirectory(true); // Retry with overwrite
                } else if (option === "rename") {
                  // Add a suffix to make the name unique (max 100 attempts)
                  const originalName = this.name;
                  const maxAttempts = 100;
                  let success = false;
                  for (let counter = 1; counter <= maxAttempts && !success; counter++) {
                    try {
                      const newName = counter === 1 ? `${originalName} (1)` : `${originalName} (${counter})`;
                      const parentPath = this.parentInfo.path;
                      const source = this.parentInfo.source;
                      const newPath = url.joinPath(parentPath, newName) + "/";

                      if (getters.isShare()) {
                        await resourcesApi.postPublic(state.shareInfo?.hash, newPath, "", false, undefined, {}, true);
                        mutations.setReload(true);
                        mutations.closeTopHover();
                        success = true;
                        return;
                      }
                      await resourcesApi.post(source, newPath, "", false, undefined, {}, true);
                      mutations.setReload(true);
                      mutations.closeTopHover();
                      success = true;

                      // Show success notification with "go to item" button
                      const buttonAction = () => {
                        url.goToItem(source, newPath, {});
                      };
                      const buttonProps = {
                        icon: "folder",
                        buttons: [{
                          label: this.$t("buttons.goToItem"),
                          primary: true,
                          action: buttonAction
                        }]
                      };
                      notify.showSuccess(this.$t("prompts.newDirSuccess"), buttonProps);
                    } catch (renameError) {
                      if (renameError.message === "conflict") {
                        // Continue to next iteration
                        continue;
                      } else {
                        throw renameError;
                      }
                    }
                  }
                  if (!success) {
                    throw new Error("Could not find a unique name after " + maxAttempts + " attempts");
                  }
                }
              } catch (retryError) {
                console.error(retryError);
              } finally {
                this.creating = false;
              }
            },
          });
        } else {
          this.creating = false;
          throw error;
        }
      }
    },
  },
};
</script>

<style scoped>
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
