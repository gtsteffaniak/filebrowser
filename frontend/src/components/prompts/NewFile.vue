<template>
  <div class="card-title">
    <h2>{{ $t("prompts.newFile") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.newFileMessage") }}</p>
    <input class="input" aria-label="FileName Field" v-focus type="text" @keyup.enter="submit"
      v-model.trim="name" />
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button class="button button--flat" @click="submit" :aria-label="$t('buttons.create')"
      :title="$t('buttons.create')">
      {{ $t("buttons.create") }}
    </button>
  </div>
</template>
<script>
import { state } from "@/store";
import { filesApi } from "@/api";
import { getters, mutations } from "@/store"; // Import your custom store
import { notify } from "@/notify";
import { url } from "@/utils";

export default {
  name: "new-file",
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
    closeHovers() {
      return mutations.closeHovers;
    },
  },
  methods: {
    async submit(event) {
      try {
        event.preventDefault();
        if (this.name === "") return;
        await this.createFile(false);
      } catch (error) {
        notify.showError(error);
      }
    },

    async createFile(overwrite = false) {
      try {
        await filesApi.post(state.req.source, url.joinPath(state.req.path, this.name), "", overwrite);
        url.goToItem(state.req.source, url.joinPath(state.req.path, this.name));
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
                  await this.createFile(true); // Retry with overwrite
                } else if (option === "rename") {
                  // Add a suffix to make the name unique (max 100 attempts)
                  const originalName = this.name;
                  const maxAttempts = 100;
                  let success = false;
                  for (let counter = 1; counter <= maxAttempts && !success; counter++) {
                    try {
                      const newName = counter === 1 ? `${originalName} (1)` : `${originalName} (${counter})`;
                      await filesApi.post(state.req.source, url.joinPath(state.req.path, newName), "", false);
                      url.goToItem(state.req.source, url.joinPath(state.req.path, newName));
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
                notify.showError(retryError);
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
