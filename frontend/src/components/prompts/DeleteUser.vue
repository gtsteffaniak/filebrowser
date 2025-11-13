<template>
  <div class="card-content">
    <p>{{ $t('prompts.deleteUserMessage') }}</p>
  </div>
  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" v-focus aria-label="Cancel"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button class="button button--flat" aria-label="Confirm Delete" @click="deleteUser">
      {{ $t("general.delete") }}
    </button>
  </div>
</template>
<script>
import { usersApi } from "@/api";
import { notify } from "@/notify";
import { mutations, getters } from "@/store";
import { eventBus } from "@/store/eventBus";

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
        await usersApi.remove(this.user.id);
        // Emit event to refresh user list
        eventBus.emit('usersChanged');
        notify.showSuccessToast(this.$t("settings.userDeleted"));
        mutations.closeHovers();
      } catch (e) {
        e.message === "403"
          ? notify.showError(this.$t("errors.forbidden"), false)
          : notify.showError(e);
      }
    },
    closeHovers() {
      mutations.closeHovers();
    }
  },
};
</script>
