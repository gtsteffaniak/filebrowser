<template>
  <div class="card-title">
    <h2>{{ $t("access.accessManagement") }}</h2>
  </div>
  <div class="card-content">
    <div v-if="isEditingPath">
      <file-list @update:selected="updateTempPath" :browse-source="sourceName"></file-list>
      <div style="margin-top: 1em; text-align: right;">
        <button class="button button--flat" @click="cancelPathChange">
          {{ $t("buttons.cancel") }}
        </button>
        <button class="button button--flat" @click="confirmPathChange">
          {{ $t("buttons.ok") }}
        </button>
      </div>
    </div>
    <div v-else>
      <p>{{ $t("prompts.source", { suffix: ":" }) }} {{ currentSource }}</p>
      <div aria-label="access-path" class="searchContext clickable button" @click="startPathEdit">
        {{ $t("search.path") }} {{ currentPath }}
      </div>
      <!-- Default behavior banner -->
      <div class="card item">
        <div class="card-content banner-content">
          <i class="material-icons">{{ sourceDenyDefault ? 'block' : 'check_circle' }}</i>  <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          {{ $t("access.defaultBehavior", { suffix: ":" }) }} {{ sourceDenyDefault ? $t("access.deny") : $t("access.allow")
          }}
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('access.defaultBehaviorDescription'))" @mouseleave="hideTooltip">
            help
          </i>
        </div>

      </div>
      <!-- Add Form -->
      <div class="form-flex-group">
        <select class="input flat-right form-compact" v-model="addType">
          <option value="user">{{ $t("general.user") }}</option>
          <option value="group">{{ $t("general.group") }}</option>
          <option value="all">{{ $t("access.all") }}</option>
        </select>
        <select v-if="addType !== 'all'" class="input flat-right flat-left form-compact" v-model="addListType">
          <option value="deny">{{ $t("access.deny") }}</option>
          <option value="allow">{{ $t("access.allow") }}</option>
        </select>
        <input v-if="addType !== 'all'" class="input flat-right flat-left form-grow form-compact" v-model="addName"
          :placeholder="$t('access.enterName')" list="group-suggestions" />
        <datalist id="group-suggestions">
          <option v-for="group in groups" :key="group" :value="group"></option>
        </datalist>
        <button class="button form-button flat-left form-compact" @click="submitAdd">
          <i class="material-icons">add</i>
        </button>
      </div>
      <table v-if="entries.length > 0">
        <tbody>
          <tr>
            <th>{{ $t("access.allowDeny") }}</th>
            <th>{{ $t("access.userGroup") }}</th>
            <th>{{ $t("general.name") }}</th>
            <th>{{ $t("buttons.edit") }}</th>
          </tr>
          <tr v-for="entry in entries" :key="entry.type + '-' + entry.name">
            <td>{{ entry.allow ? $t("access.allow") : $t("access.deny") }}</td>
            <td>{{ entry.type === 'user' ? $t("general.user") : (entry.type === 'group' ? $t("general.group") :
              $t('access.all')) }}</td>
            <td>{{ entry.name }}</td>
            <td>
              <button @click="deleteAccess(entry)" class="action" :aria-label="$t('buttons.delete')"
                :title="$t('buttons.delete')">
                <i class="material-icons">delete</i>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
  <div class="card-action">
    <button v-if="!isEditingPath" @click="closeHovers" class="button button--flat button--grey"
      :aria-label="$t('buttons.close')" :title="$t('buttons.close')">
      {{ $t("buttons.close") }}
    </button>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { accessApi } from "@/api";
import { mutations } from "@/store";
import FileList from "./FileList.vue";

export default {
  name: "access",
  components: { FileList },
  props: {
    sourceName: { type: String, required: true },
    path: { type: String, required: true, default: "/" }
  },
  data() {
    return {
      isEditingPath: false,
      tempPath: this.path,
      currentPath: this.path,
      currentSource: this.sourceName,
      tempSource: this.sourceName,
      rule: { denyAll: false, deny: { users: [], groups: [] }, allow: { users: [], groups: [] } },
      sourceDenyDefault: false,
      addType: "user",
      addListType: "deny",
      addName: "",
      groups: []
    };
  },
  computed: {
    entries() {
      /** @type {{allow: boolean, type: "user" | "group" | "all", name: string}[]} */
      const entries = [];
      if (this.rule.denyAll) {
        entries.push({ allow: false, type: "all", name: this.$t("access.all") });
      }
      (this.rule.deny?.users || []).forEach(name => {
        entries.push({ allow: false, type: "user", name });
      });
      (this.rule.deny?.groups || []).forEach(name => {
        entries.push({ allow: false, type: "group", name });
      });
      (this.rule.allow?.users || []).forEach(name => {
        entries.push({ allow: true, type: "user", name });
      });
      (this.rule.allow?.groups || []).forEach(name => {
        entries.push({ allow: true, type: "group", name });
      });
      return entries;
    }
  },
  async mounted() {
    await this.fetchRule();
    await this.fetchGroups();
  },
  watch: {
    sourceName(newSourceName) {
      this.currentSource = newSourceName;
      this.tempSource = newSourceName;
      this.fetchRule();
    },
    path(newPath) {
      this.currentPath = newPath;
      this.tempPath = newPath;
      this.isEditingPath = false;
      this.fetchRule();
    }
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    startPathEdit() {
      this.tempPath = this.currentPath;
      this.isEditingPath = true;
    },
    /**
     * @param {{path: string, source: string}} pathOrData
     */
    updateTempPath(pathOrData) {
      if (pathOrData && pathOrData.path) {
        this.tempPath = pathOrData.path;
        this.tempSource = pathOrData.source;
      }
    },
    confirmPathChange() {
      this.currentPath = this.tempPath;
      this.currentSource = this.tempSource;
      this.isEditingPath = false;
      this.fetchRule();
    },
    cancelPathChange() {
      this.isEditingPath = false;
    },
    async fetchGroups() {
      try {
        const response = await accessApi.getGroups();
        this.groups = response.groups;
      } catch (e) {
        notify.showError(e);
      }
    },
    async fetchRule() {
      try {
        const response = await accessApi.get(this.currentSource, this.currentPath);
        // Handle new API response structure - now sourceDenyDefault is part of the rule
        this.rule = response;
        this.sourceDenyDefault = response.sourceDenyDefault || false;
      } catch (e) {
        notify.showError(e);
        this.rule = { denyAll: false, deny: { users: [], groups: [] }, allow: { users: [], groups: [] } };
        this.sourceDenyDefault = false;
      }
    },
    /**
     * @param {{allow: boolean, type: string, name: string}} entry
     */
    async deleteAccess(entry) {
      try {
        const body = {
          allow: entry.allow,
          ruleCategory: entry.type,
          value: entry.type === 'all' ? '' : entry.name,
        };
        await accessApi.del(this.currentSource, this.currentPath, body);
        notify.showSuccess(this.$t("access.deleted"));
        await this.fetchRule();
        this.$emit('updated');
      } catch (e) {
        notify.showError(e);
      }
    },
    async submitAdd() {
      if (!this.addName.trim() && this.addType !== "all") {
        notify.showError(this.$t("access.enterName"));
        return;
      }
      try {
        const body = {
          allow: this.addListType === 'allow' && this.addType !== 'all',
          ruleCategory: this.addType,
          value: this.addName.trim()
        };
        await accessApi.add(
          this.currentSource,
          this.currentPath,
          body
        );
        notify.showSuccess(this.$t("access.added"));
        this.addName = "";
        await this.fetchRule();
        this.$emit('updated');
      } catch (e) {
        notify.showError(e);
      }
    },
    closePrompt() {
      if (mutations && mutations.closeHovers) {
        mutations.closeHovers();
      } else {
        this.$emit('close');
      }
    },
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    }
  }
};
</script>

<style scoped>
.form-flex-group {
  margin-top: 1em;
}

.banner-content {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0.25em !important;
  gap: 0.5em;
}

.searchContext {
  margin-bottom: 1em;
}
</style>