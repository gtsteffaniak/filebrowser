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
import { mapGetters, mapMutations, mapState } from "vuex";
import { users as api } from "@/api";
import buttons from "@/utils/buttons";

export default {
  name: "delete",
  computed: {
    ...mapState(["prompts"]),
    currentPrompt() {
      return this.prompts.length ? this.prompts[this.prompts.length - 1] : null;
    },
    user() {
      return this.currentPrompt?.props?.user;
    }
  },
  methods: {
    async deleteUser(event) {
      event.preventDefault();
      try {
        await api.remove(this.user.id);
        this.$router.push({ path: "/settings/users" });
        this.$showSuccess(this.$t("settings.userDeleted"));
      } catch (e) {
        e.message === "403"
          ? this.$showError(this.$t("errors.forbidden"), false)
          : this.$showError(e);
      }
    },
    ...mapMutations(["closeHovers"]),
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

        if (this.selectedCount === 0) {
          return;
        }

        let promises = [];
        for (let index of this.selected) {
          promises.push(api.remove(this.req.items[index].url));
        }

        await Promise.all(promises);
        buttons.success("delete");
        this.$store.commit("setReload", true);
      } catch (e) {
        buttons.done("delete");
        this.$showError(e);
        if (this.isListing) this.$store.commit("setReload", true);
      }
    },
  },
};
</script>
