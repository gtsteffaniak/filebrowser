<template>
  <div class="card-content">
    <p v-if="isNew">
      <label for="group-name">{{ $t("access.groupName") }}</label>
      <input
        id="group-name"
        class="input"
        :class="{ 'form-invalid': nameError }"
        type="text"
        v-model="name"
        :placeholder="$t('access.groupName')"
        v-focus
        @keyup.enter="submit"
      />
      <span v-if="nameError" class="name-error">{{ nameError }}</span>
    </p>
    <div v-else class="info-item">
      <strong>{{ $t("access.groupName") }}</strong>
      <span aria-label="group name">{{ name }}</span>
    </div>
    <label class="members-label">{{ membersLabel }}</label>
    <EntityPickerList
      kind="user"
      multiple
      v-model="selected"
      :extra-options="selected"
    />
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="closeTopPrompt"
      :aria-label="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      :disabled="!name || saving"
      @click="submit"
      :aria-label="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { accessApi } from "@/api";
import { mutations } from "@/store";
import { notify } from "@/notify";
import { eventBus } from "@/store/eventBus";
import EntityPickerList from "./EntityPickerList.vue";
import { groupNameErrorText } from "@/utils/groups.js";

export default {
  name: "group-edit",
  components: { EntityPickerList },
  props: {
    promptId: { type: [String, Number], default: null },
    group: { type: String, default: "" },
    members: { type: Array, default: () => [] },
    /** Existing group names used to block duplicate creation. */
    existingGroups: { type: Array, default: () => [] },
  },
  data() {
    return {
      name: this.group,
      selected: [...this.members],
      saving: false,
    };
  },
  computed: {
    membersLabel() {
      return `${this.$t("access.groupMembers")} (${this.selected.length})`;
    },
    isNew() {
      return !this.group;
    },
    /** Inline validation message for the name field (new groups only). */
    nameError() {
      if (!this.isNew || !this.name) {
        return null;
      }
      const message = groupNameErrorText(this.name);
      if (message) {
        return message;
      }
      if (this.existingGroups.includes(this.name.trim())) {
        return this.$t("access.groupExists");
      }
      return null;
    },
  },
  methods: {
    closeTopPrompt() {
      mutations.closeTopPrompt(this.promptId ?? undefined);
    },
    async submit() {
      if (!this.name || this.nameError || this.saving) return;
      this.saving = true;
      try {
        await accessApi.saveGroup(this.name.trim(), this.selected, this.isNew);
        eventBus.emit("groupsChanged");
        mutations.closeTopPrompt(this.promptId ?? undefined);
      } catch (e) {
        notify.showError(e);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.info-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75em;
  padding: 0.5em;
  margin-bottom: 0.5em;
  border-radius: var(--borderRadius);
  transition: background-color 0.2s;
}

.info-item:hover {
  background-color: var(--hoverOverlay);
}

.info-item strong {
  min-width: 120px;
  font-weight: 600;
  color: var(--textPrimary);
}

.info-item span {
  flex: 1;
  color: var(--textSecondary);
  overflow-wrap: break-word;
}

.members-label {
  display: block;
  margin-bottom: 0.5em;
}

.name-error {
  color: var(--red);
  font-size: 0.85em;
}
</style>
