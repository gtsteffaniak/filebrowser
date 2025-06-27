<template>
  <div class="card floating share__promt__card" id="access">
    <div class="card-title">
      <h2>{{ $t("buttons.access") }}</h2>
    </div>
    <div aria-label="access-path" class="searchContext">
      {{ $t("search.path") }} {{ path }}
    </div>
    <div class="card-content">
      <!-- Add Form -->
      <div class="add-form" style="margin-bottom: 1em;">
        <select v-model="addType">
          <option value="user">User</option>
          <option value="group">Group</option>
        </select>
        <select v-model="addListType">
          <option value="deny">Deny</option>
          <option value="allow">Allow</option>
        </select>
        <input v-model="addName" :placeholder="$t('settings.enterName')" />
        <button class="action" @click="submitAdd">
          <i class="material-icons">add</i>
          {{ $t("buttons.add") }}
        </button>
      </div>
      <table>
        <tbody>
          <tr>
            <th>allow/deny</th>
            <th>user/group</th>
            <th>name</th>
            <th>actions</th>
          </tr>
          <tr v-for="entry in entries" :key="entry.type + '-' + entry.name">
            <td>{{ entry.allow ? 'allow' : 'deny' }}</td>
            <td>{{ entry.type }}</td>
            <td>{{ entry.name }}</td>
            <td>
              <button @click="deleteAccess(entry)" class="action" :aria-label="$t('buttons.delete')" :title="$t('buttons.delete')">
                <i class="material-icons">delete</i>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <!-- Cancel Button -->
      <div style="margin-top: 1em; text-align: right;">
        <button class="action" @click="closePrompt">
          <i class="material-icons">close</i>
          {{ $t("buttons.cancel") }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { accessApi } from "@/api";
import { mutations } from "@/store";

export default {
  name: "access",
  props: {
    sourceName: { type: String, required: true },
    path: { type: String, required: true }
  },
  data() {
    return {
      rule: { deny: { users: [], groups: [] }, allow: { users: [], groups: [] } },
      addType: "user",
      addListType: "deny",
      addName: ""
    };
  },
  computed: {
    entries() {
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
    sourceName: 'fetchRule',
    path: 'fetchRule'
  },
  methods: {
    async fetchRule() {
      try {
        this.rule = await accessApi.get(this.sourceName, this.path);
      } catch (e) {
        notify.showError(e);
        this.rule = { deny: { users: [], groups: [] }, allow: { users: [], groups: [] } };
      }
    },
    async deleteAccess(entry) {
      try {
        const body = {
          allow: entry.allow,
          ruleCategory: entry.type,
          value: entry.name
        };
        await accessApi.del(this.sourceName, this.path, body);
        notify.showSuccess(this.$t("success.deleted"));
        await this.fetchRule();
        this.$emit('updated');
      } catch (e) {
        notify.showError(e);
      }
    },
    async submitAdd() {
      if (!this.addName.trim()) {
        notify.showError(this.$t("settings.enterName"));
        return;
      }
      try {
        const body = {
          allow: this.addListType === 'allow',
          ruleCategory: this.addType,
          value: this.addName.trim()
        };
        await accessApi.add(
          this.sourceName,
          this.path,
          body
        );
        notify.showSuccess(this.$t("success.added"));
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
