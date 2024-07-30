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
          :disabled="$route.path === dest"
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
import { files as api } from "@/api";
import buttons from "@/utils/buttons";
import * as upload from "@/utils/upload";
import { showError } from "@/notify";

export default {
  name: "move",
  components: { FileList },
  data: function () {
    return {
      current: window.location.pathname,
      dest: null,
    };
  },
  computed: {
    user() {
      return state.user;
    },
    closeHovers() {
      return mutations.closeHovers()
    },
  },
  methods: {
    move: async function (event) {
      event.preventDefault();
      let items = [];

      for (let item of state.selected) {
        items.push({
          from: state.req.items[item].url,
          to: this.dest + encodeURIComponent(state.req.items[item].name),
          name: state.req.items[item].name,
        });
      }

      let action = async (overwrite, rename) => {
        buttons.loading("move");
        await api
          .move(items, overwrite, rename)
          .then(() => {
            buttons.success("move");
            this.$router.push({ path: this.dest });
            mutations.setReload(true)
          })
          .catch((e) => {
            buttons.done("move");
            showError(e);
          });
      };

      let dstItems = (await api.fetch(this.dest)).items;
      let conflict = upload.checkConflict(items, dstItems);

      let overwrite = false;
      let rename = false;

      if (conflict) {
        mutations.showHover({
          name: "replace-rename",
          confirm: (event, option) => {
            overwrite = option == "overwrite";
            rename = option == "rename";

            event.preventDefault();
            mutations.closeHovers();
            action(overwrite, rename);
            mutations.setReload(true)
          },
        });

        return;
      }

      action(overwrite, rename);
    },
  },
};
</script>
