<template>
    <div class="card-content">
      <p v-if="selectedCount === 1">
        {{ $t("prompts.deleteMessageSingle") }}
      </p>
      <p v-else>
        {{ $t("prompts.deleteMessageMultiple", { count: selectedCount }) }}
      </p>
      <div style="display: grid" aria-label="delete-path" class="searchContext button">
        <span v-for="(item, index) in nav" :key="index"> {{ item }} </span>
      </div>
    </div>
    <div class="card-action">
      <button @click="closeHovers" class="button button--flat button--grey" :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')">
        {{ $t("buttons.cancel") }}
      </button>
      <button @click="submit" class="button button--flat button--red" aria-label="Confirm-Delete"
        :title="$t('buttons.delete')">
        {{ $t("buttons.delete") }}
      </button>
    </div>
</template>

<script>
import { filesApi } from "@/api";
import buttons from "@/utils/buttons";
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";

export default {
  name: "delete",
  mounted() {
    const selectedCount = getters.selectedCount();
    if (selectedCount > 0) {
      if (state.user.deleteWithoutConfirming && getters.isSingleFileSelected()) {
        this.submit();
      }
    }

  },
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
      if (state.isSearchActive || getters.currentView() == "preview") {
        return [state.selected[0].path];
      }
      let paths = [];
      for (let index of state.selected) {
        paths.push(state.req.items[index].path);
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
        const currentView = getters.currentView()
        if (state.isSearchActive || currentView == "preview") {
          await filesApi.remove(state.selected[0].source, state.selected[0].path);
          buttons.success("delete");
          notify.showSuccess("Deleted item successfully");
          mutations.closeHovers();
          mutations.setDeletedItem(true);
          mutations.setReload(true);
          return;
        }
        if (!this.isListing) {
          await filesApi.remove(state.req.items.source, state.req.items[state.selected[0]].path);
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
          promises.push(filesApi.remove(state.req.source, state.req.items[index].path));
        }
        mutations.resetSelected();
        await Promise.all(promises);
        buttons.success("delete");
        notify.showSuccess(this.$t('prompts.deleted'));
        mutations.setReload(true); // Handle reload as neededs
      } catch (e) {
        buttons.done("delete");
        notify.showError(e);
        if (this.isListing) mutations.setReload(true); // Handle reload as needed
      }
    },
  },
};
</script>
