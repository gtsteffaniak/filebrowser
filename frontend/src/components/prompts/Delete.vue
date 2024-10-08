<template>
  <div class="card floating">
    <div class="card-content">
      <p v-if="selectedCount === 1">
        {{ $t("prompts.deleteMessageSingle") }}
      </p>
      <p v-else>
        {{ $t("prompts.deleteMessageMultiple", { count: selectedCount }) }}
      </p>
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
import { files as api } from "@/api";
import buttons from "@/utils/buttons";
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";

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
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async submit() {
      buttons.loading("delete");

      try {
        if (!this.isListing) {
          await api.remove(state.route.path);
          buttons.success("delete");
          showSuccess("Deleted item successfully");

          this.currentPrompt?.confirm();
          this.closeHovers();
          return;
        }

        this.closeHovers();

        if (getters.selectedCount() === 0) {
          return;
        }

        let promises = [];
        for (let index of state.selected) {
          promises.push(api.remove(state.req.items[index].url));
        }

        await Promise.all(promises);
        buttons.success("delete");
        showSuccess("Deleted item successfully");
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
