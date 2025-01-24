<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.move") }}</h2>
    </div>

    <div class="card-content">
      <file-list ref="fileList" @update:selected="(val) => (dest = val)"> </file-list>
    </div>

    <div
      class="card-action"
      style="display: flex; align-items: center; justify-content: space-between"
    >
      <template v-if="user.perm.create">
        <button
          class="button button--flat"
          @click="$refs.fileList.createDir()"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
          style="justify-self: left"
        >
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>
      </template>
      <div>
        <button
          class="button button--flat button--grey"
          @click="closeHovers"
          :aria-label="$t('buttons.cancel')"
          :title="$t('buttons.cancel')"
        >
          {{ $t("buttons.cancel") }}
        </button>
        <button
          class="button button--flat"
          @click="move"
          :aria-label="$t('buttons.move')"
          :title="$t('buttons.move')"
        >
          {{ $t("buttons.move") }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import FileList from "./FileList.vue";
import { filesApi } from "@/api";
import buttons from "@/utils/buttons";
import * as upload from "@/utils/upload";
import { removePrefix } from "@/utils/url";
import { notify } from "@/notify";

export default {
  name: "move",
  components: { FileList },
  data: function () {
    return {
      current: window.location.pathname,
      dest: null,
      items: [],
    };
  },
  computed: {
    user() {
      return state.user;
    },
    closeHovers() {
      return mutations.closeHovers();
    },
  },
  mounted() {
    if (state.isSearchActive) {
      this.items = [
        {
          from: "/files" + state.selected[0].url,
          name: state.selected[0].name,
        },
      ];
    } else {
      for (let item of state.selected) {
        this.items.push({
          from: state.req.items[item].url,
          // add to: dest
          name: state.req.items[item].name,
        });
      }
    }
  },
  methods: {
    move: async function (event) {
      event.preventDefault();
      try {
        // Define the action function
        let action = async (overwrite, rename) => {
          for (let item of this.items) {
            item.to = this.dest + item.name;
          }
          buttons.loading("move");
          await filesApi.moveCopy(this.items, "move", overwrite, rename);
        };
        // Fetch destination files
        let dstResp = await filesApi.fetchFiles(this.dest);
        let conflict = upload.checkConflict(this.items, dstResp.items);
        let overwrite = false;
        let rename = false;

        if (conflict) {
          await new Promise((resolve, reject) => {
            mutations.showHover({
              name: "replace-rename",
              confirm: async (event, option) => {
                overwrite = option == "overwrite";
                rename = option == "rename";
                event.preventDefault();
                try {
                  await action(overwrite, rename);
                  resolve(); // Resolve the promise if action succeeds
                } catch (e) {
                  reject(e); // Reject the promise if an error occurs
                }
              },
            });
          });
        } else {
          // Await the action call for non-conflicting cases
          await action(overwrite, rename);
        }
        mutations.closeHovers();
        mutations.setSearch(false);
        notify.showSuccess("Successfully moved file/folder, redirecting...");
        setTimeout(() => {
          this.$router.push(this.dest);
        }, 1000);
      } catch (error) {
        notify.showError(error);
      }
    },
  },
};
</script>
