<template>
  <div class="card-title">
    <h2>{{ $t("prompts.newDir") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.newDirMessage") }}</p>
    <input aria-label="New Folder Name" class="input" type="text" @keyup.enter="submit" v-model.trim="name"
      v-focus />
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button class="button button--flat" :aria-label="$t('buttons.create')" :title="$t('buttons.create')"
      @click="submit">
      {{ $t("buttons.create") }}
    </button>
  </div>
</template>
<script>
import { filesApi, publicApi } from "@/api";
import { getters, mutations, state } from "@/store"; // Import your custom store
import { goToItem } from "@/utils/url";
import { url } from "@/utils";
import { shareInfo } from "@/utils/constants";
export default {
  name: "new-dir",
  props: {
    redirect: {
      type: Boolean,
      default: true,
    },
    base: {
      type: [String, null],
      default: null,
    },
  },
  data() {
    return {
      name: "",
    };
  },
  computed: {
    isFiles() {
      return getters.isFiles();
    },
    isListing() {
      return getters.isListing();
    },
  },
  methods: {
    closeHovers() {
      return mutations.closeHovers();
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
      try {
        if (getters.isShare()) {
          await publicApi.post(shareInfo.hash, url.joinPath(state.req.path, this.name) + "/", "", overwrite);
          goToItem(state.req.source, url.joinPath(state.req.path, this.name), {});
          mutations.closeHovers();
          return;
        }
        await filesApi.post(state.req.source, url.joinPath(state.req.path, this.name) + "/", "", overwrite);
        goToItem(state.req.source, url.joinPath(state.req.path, this.name), {});
        mutations.closeHovers();
      } catch (error) {
        if (error.message === "conflict") {
          // Show replace-rename prompt for file/folder conflicts
          mutations.showHover({
            name: "replace-rename",
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
                      if (getters.isShare()) {
                        await publicApi.post(shareInfo.hash, url.joinPath(state.req.path, newName) + "/", "", false);
                        goToItem(state.req.source, url.joinPath(state.req.path, newName), {});
                        mutations.closeHovers();
                        success = true;
                        return;
                      }
                      await filesApi.post(state.req.source, joinPath(state.req.path, newName) + "/", "", false);
                      goToItem(state.req.source, joinPath(state.req.path, newName), {});
                      mutations.closeHovers();
                      success = true;
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
              }
            },
          });
        } else {
          throw error;
        }
      }
    },
  },
};
</script>
