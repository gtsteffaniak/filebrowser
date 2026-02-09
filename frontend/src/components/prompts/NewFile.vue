<template>
  <div class="card-content">
    <p>{{ $t("prompts.newFileMessage") }}</p>
    <input class="input" aria-label="FileName Field" v-focus type="text" @keyup.enter="submit"
      v-model.trim="name" />
  </div>

  <div class="card-actions">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button class="button button--flat" @click="submit" :aria-label="$t('general.create')"
      :title="$t('general.create')">
      {{ $t("general.create") }}
    </button>
  </div>
</template>
<script>
import { state } from "@/store";
import { filesApi, publicApi } from "@/api";
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
        console.error(error);
      }
    },

    async createFile(overwrite = false) {
      try {
        const newPath = url.joinPath(state.req.path, this.name);
        const source = state.req.source;

        if (getters.isShare()) {
          await publicApi.post(state.shareInfo?.hash, newPath, "", overwrite);
          mutations.setReload(true);
          mutations.closeHovers();
          return;
        }
        await filesApi.post(source, newPath, "", overwrite);
        mutations.setReload(true);
        mutations.closeHovers();

        // Show success notification with "go to item" button
        const buttonAction = () => {
          url.goToItem(source, newPath, {});
        };
        const buttonProps = {
          icon: "insert_drive_file",
          buttons: [{
            label: this.$t("buttons.goToItem"),
            primary: true,
            action: buttonAction
          }]
        };
        notify.showSuccess(this.$t("prompts.newFileSuccess"), buttonProps);
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
                      const newPath = url.joinPath(state.req.path, newName);
                      const source = state.req.source;

                      if (getters.isShare()) {
                        await publicApi.post(state.shareInfo?.hash, newPath, "", false);
                        mutations.setReload(true);
                        mutations.closeHovers();
                        success = true;
                        return;
                      }
                      await filesApi.post(source, newPath, "", false);
                      mutations.setReload(true);
                      mutations.closeHovers();
                      success = true;

                      // Show success notification with "go to item" button
                      const buttonAction = () => {
                        url.goToItem(source, newPath, {});
                      };
                      const buttonProps = {
                        icon: "insert_drive_file",
                        buttons: [{
                          label: this.$t("buttons.goToItem"),
                          primary: true,
                          action: buttonAction
                        }]
                      };
                      notify.showSuccess(this.$t("prompts.newFileSuccess"), buttonProps);
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
