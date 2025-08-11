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
      <div aria-label="access-path" class="searchContext" @click="startPathEdit" style="cursor: pointer;">
        {{ $t("search.path") }} {{ currentPath }}
      </div>
      <!-- Add Form -->
      <div class="form-flex-group" >
        <select class="input flat-right form-compact" v-model="addType">
          <option value="user">{{ $t("general.user") }}</option>
          <option value="group">{{ $t("general.group") }}</option>
        </select>
        <select class="input flat-right flat-left form-compact" v-model="addListType">
          <option value="deny">{{ $t("access.deny") }}</option>
          <option value="allow">{{ $t("access.allow") }}</option>
        </select>
        <input class="input flat-right flat-left form-grow form-compact" v-model="addName" :placeholder="$t('access.enterName')" />
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
            <td>{{ entry.type == "user" ? $t("general.user") : $t("general.group") }}</td>
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
      <button @click="closeHovers" class="button button--flat button--grey" :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')">
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
      rule: { deny: { users: [], groups: [] }, allow: { users: [], groups: [] } },
      addType: "user",
      addListType: "deny",
      addName: ""
    };
  },
  computed: {
    entries() {
      /** @type {{allow: boolean, type: "user" | "group", name: string}[]} */
      const entries = [];
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
    async fetchRule() {
      try {
        this.rule = await accessApi.get(this.currentSource, this.currentPath);
      } catch (e) {
        notify.showError(e);
        this.rule = { deny: { users: [], groups: [] }, allow: { users: [], groups: [] } };
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
          value: entry.name
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
      if (!this.addName.trim()) {
        notify.showError(this.$t("access.enterName"));
        return;
      }
      try {
        const body = {
          allow: this.addListType === 'allow',
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
    }
  }
};
</script>
