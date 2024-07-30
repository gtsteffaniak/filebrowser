<template>
  <div class="card floating">
    <div class="card-content">
      <p>Are you sure you want to delete this user?</p>
    </div>
    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        v-focus
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button class="button button--flat" @click="deleteUser">
        {{ $t("buttons.delete") }}
      </button>
    </div>
  </div>
</template>
<script>
import { users as api } from "@/api";
import { showSuccess,showError } from "@/notify";
import buttons from "@/utils/buttons";
import { state, mutations, getters } from "@/store";

export default {
  name: "delete",
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
    user() {
      return this.currentPrompt?.props?.user;
    },
  },
  methods: {
    async deleteUser(event) {
      event.preventDefault();
      try {
        await api.remove(this.user.id);
        this.$router.push({ path: "/settings/users" });
        showSuccess(this.$t("settings.userDeleted"));
      } catch (e) {
        e.message === "403"
          ? showError(this.$t("errors.forbidden"), false)
          : showError(e);
      }
    },
    closeHovers() {
      mutations.closeHovers();
    },
    submit: async function () {
      buttons.loading("delete");

      try {
        if (!this.isListing) {
          await api.remove(this.$route.path);
          buttons.success("delete");

          this.currentPrompt?.confirm();
          this.closeHovers();
          return;
        }

        this.closeHovers();

        if (getters.selectedCount() === 0) {
          return;
        }

        let promises = [];
        for (let index of this.selected) {
          promises.push(api.remove(state.req.items[index].url));
        }

        await Promise.all(promises);
        buttons.success("delete");
        mutations.setReload(true); // Handle reload as needed
      } catch (e) {
        buttons.done("delete");
        showError(e);
        if (this.isListing) mutations.setReload(true); // Handle reload as needed
      }
    },
  },
};
</script>
