<template>
  <div class="card floating">
    <div class="card-content">
      <p v-if="selectedCount === 1">
        {{ $t("prompts.deleteMessageSingle") }}
      </p>
      <p v-else>
        {{ $t("prompts.deleteMessageMultiple", { count: selectedCount }) }}
      </p>
      <div style="display: grid" class="searchContext">
        <span v-for="item in nav"> {{ item }} </span>
      </div>
    </div>
    <div class="card-action">
      <button
        @click="closeHovers"
        class="button button--flat button--grey"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        @click="submit"
        class="button button--flat button--red"
        :aria-label="$t('buttons.delete')"
        :title="$t('buttons.delete')"
      >
        {{ $t("buttons.delete") }}
      </button>
    </div>
  </div>
</template>

<script>
import { filesApi } from "@/api";
import buttons from "@/utils/buttons";
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";
import { removePrefix } from "@/utils/url";

export default {
  name: "delete",
  computed: {
    isListing() {
      return getters.isListing();
    },
    selectedCount() {
      return getters.selectedCount();
    },
    currentPrompt() {
      return getters.currentPrompt();
    },
    nav() {
      if (state.isSearchActive) {
        return [state.selected[0].path];
      }
      let paths = [];
      for (let index of state.selected) {
        paths.push(removePrefix(state.req.items[index].url, "files"));
      }
      return paths;
    },
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async submit() {
      buttons.loading("delete");

      try {
        if (state.isSearchActive) {
          await filesApi.remove(state.selected[0].url);
          buttons.success("delete");
          notify.showSuccess("Deleted item successfully");
          mutations.closeHovers();
          return;
        }
        if (!this.isListing) {
          await filesApi.remove(state.route.path);
          buttons.success("delete");
          notify.showSuccess("Deleted item successfully");

          this.currentPrompt?.confirm();
          mutations.closeHovers();
          return;
        }

        mutations.closeHovers();

        if (getters.selectedCount() === 0) {
          return;
        }

        let promises = [];
        for (let index of state.selected) {
          promises.push(filesApi.remove(state.req.items[index].url));
        }

        await Promise.all(promises);
        buttons.success("delete");
        notify.showSuccess("Deleted item successfully");
        window.location.reload();
        mutations.setReload(true); // Handle reload as needed
      } catch (e) {
        buttons.done("delete");
        notify.showError(e);
        if (this.isListing) mutations.setReload(true); // Handle reload as needed
      }
    },
  },
};
</script>
