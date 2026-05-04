<template>
  <div>
    <h4>{{ $t("profileSettings.passkeys") }}</h4>
    <p class="description">{{ $t("profileSettings.passkeysDescription") }}</p>

    <div v-if="passkeys.length > 0" class="passkey-list">
      <div v-for="pk in passkeys" :key="pk.id" class="passkey-item">
        <div class="passkey-info">
          <span class="passkey-name">{{ pk.name || $t("profileSettings.passkeyDefaultName") }}</span>
          <span class="passkey-meta">
            {{ $t("profileSettings.created") }} {{ formatDate(pk.createdAt) }}<span v-if="pk.lastUsedAt" class="passkey-last-used">{{ $t("profileSettings.lastUsed") }} {{ formatDate(pk.lastUsedAt) }}</span>
          </span>
        </div>
        <button type="button" class="button button--flat button--red" @click="deletePasskey(pk.id)">
          {{ $t("general.delete") }}
        </button>
      </div>
    </div>
    <div v-else class="passkey-empty">
      {{ $t("profileSettings.noPasskeys") }}
    </div>

    <button type="button" class="button button--block" @click="addPasskey" :disabled="addingPasskey">
      {{ addingPasskey ? $t("profileSettings.addingPasskey") : $t("profileSettings.addPasskey") }}
    </button>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state } from "@/store";
import { authApi } from "@/api";

export default {
  name: "passkeySettings",
  data() {
    return {
      addingPasskey: false,
    };
  },
  computed: {
    passkeys() {
      return state.user?.passkeyCredentials || [];
    },
  },
  methods: {
    formatDate(timestamp) {
      if (!timestamp) return "";
      const date = new Date(timestamp * 1000);
      return date.toLocaleDateString();
    },
    async addPasskey() {
      this.addingPasskey = true;
      try {
        await authApi.beginPasskeyRegistration();
        notify.showSuccessToast(this.$t("profileSettings.passkeyAdded"));
        setTimeout(() => { window.location.reload(); }, 500);
      } catch (err) {
        notify.showError(err.message || this.$t("profileSettings.passkeyAddFailed"));
      } finally {
        this.addingPasskey = false;
      }
    },
    async deletePasskey(id) {
      try {
        await authApi.deletePasskeyCredential(id);
        notify.showSuccessToast(this.$t("profileSettings.passkeyDeleted"));
        setTimeout(() => { window.location.reload(); }, 500);
      } catch (err) {
        notify.showError(err.message || this.$t("profileSettings.passkeyDeleteFailed"));
      }
    },
  },
};
</script>

<style scoped>
.passkey-list {
  margin-bottom: 1em;
}

.passkey-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5em 0;
  border-bottom: 1px solid var(--borderColor, #ddd);
}

.passkey-info {
  display: flex;
  flex-direction: column;
}

.passkey-name {
  font-weight: 500;
}

.passkey-meta {
  font-size: 0.8em;
  color: var(--textSecondary, #888);
}

.passkey-last-used::before {
  content: " · ";
}

.passkey-empty {
  padding: 1em 0;
  color: var(--textSecondary, #888);
  text-align: center;
}

.description {
  color: var(--textSecondary, #888);
  font-size: 0.9em;
  margin-bottom: 1em;
}
</style>
