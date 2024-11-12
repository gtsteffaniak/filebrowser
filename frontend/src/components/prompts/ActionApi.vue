<template>
  <div class="card floating">
    <div class="card-title">
      <h2>API Key Details</h2>
    </div>

    <div class="card-content">
      <button
        class="action copy-clipboard"
        :data-clipboard-text="info.key"
        :aria-label="$t('buttons.copyToClipboard')"
        :title="$t('buttons.copyToClipboard')"
      >
        API Key Name : {{ name }}
        <i class="material-icons">content_paste</i>
      </button>

      <h3>Created At :</h3>
      {{ formatTime(info.created) }}
      <h3>Expires At :</h3>
      {{ formatTime(info.expires) }}
      <h3>Permissions:</h3>
      <table>
        <tbody>
          <tr v-for="(isEnabled, perm) in info.Permissions" :key="perm">
            <td>{{ perm }}</td>
            <td>{{ isEnabled ? "✓" : "✗" }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')"
      >
        {{ $t("buttons.close") }}
      </button>
      <button
        class="button button--flat button--red"
        @click="deleteApi"
        :title="$t('buttons.delete')"
      >
        {{ $t("buttons.delete") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";

export default {
  name: "ActionApi",
  props: {
    name: {
      type: String,
      required: true,
    },
    info: {
      type: Object,
      required: true,
    },
  },
  methods: {
    formatTime(timestamp) {
      return new Date(timestamp * 1000).toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    },
    closeHovers() {
      mutations.closeHovers();
    },
    deleteApi() {
      // Dummy delete function, to be filled in later
      try {
        usersApi.deleteApiKey({ name: this.name });
        notify.showSuccess("API key deleted!");
        window.location.reload();
      } catch (error) {
        notify.showError(error);
      }
    },
  },
};
</script>

<style>
/* Basic styling for the prompt */
.card.floating {
  padding: 20px;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}
.card-content {
  margin-bottom: 16px;
}
.card-action {
  display: flex;
  justify-content: flex-end;
}
.card-action .button {
  margin-left: 8px;
}
</style>
