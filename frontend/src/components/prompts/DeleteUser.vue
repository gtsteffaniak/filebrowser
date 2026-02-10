<template>
  <div class="card-content">
    <!-- Loading spinner overlay -->
    <div v-show="deleting" class="loading-content">
      <LoadingSpinner size="small" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!deleting">
      <p>{{ $t('prompts.deleteUserMessage') }}</p>
    </div>
  </div>
  <div class="card-actions">
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
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "delete",
  components: {
    LoadingSpinner,
  },
  data() {
    return {
      deleting: false,
    };
  },
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
      this.deleting = true;
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
      } finally {
        this.deleting = false;
      }
    },
    closeHovers() {
      mutations.closeHovers();
    }
  },
};
</script>

<style scoped>
.loading-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding-top: 2em;
}

.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}

.card-content {
  position: relative;
}
</style>
