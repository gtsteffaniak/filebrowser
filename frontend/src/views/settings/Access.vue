<template>
  <button class="button floating-action-button" @click="addAccess">{{ $t("general.new") }}</button>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("access.accessManagement") }}</h2>
    <div class="form-flex-group">
      <label for="source-select">{{ $t("general.source",{suffix: ":"})  }}</label>
      <select class="input" id="source-select" v-model="selectedSource" @change="fetchRules">
        <option v-for="source in availableSources" :key="source" :value="source">
          {{ source }}
        </option>
      </select>
    </div>

  </div>
  <div class="card-content full">
    <div v-if="loading" class="loading-spinner">
      <i class="material-icons spin">sync</i>
    </div>
    <table v-else aria-label="Access Rules">
      <thead>
        <tr>
          <th>{{$t('general.path')}}</th>
          <th>{{$t('access.totalDenied')}}</th>
          <th>{{$t('access.totalAllowed')}}</th>
          <th></th>
          <th>{{$t('general.edit') }}</th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="(rule, path) in rules" :key="path">
          <td>{{ path }}</td>
          <td>{{ (rule.deny.users.length + rule.deny.groups.length) + (rule.denyAll ? 1 : 0) }}</td>
          <td>{{ rule.allow.users.length + rule.allow.groups.length }}</td>
          <td class="small">
            <i v-if="!rule.pathExists" class="material-icons warning-icon" :title="$t('messages.pathNotFound')">warning</i>
          </td>
          <td class="small">
            <button class="action" @click="editAccess(path)" :aria-label="$t('general.edit')"
              :title="$t('general.edit')">
              <i class="material-icons">edit</i>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="Object.keys(rules).length === 0 && !loading">
      <h2 class="message" v-if="Object.keys(rules).length === 0">
      <i class="material-icons">sentiment_dissatisfied</i>
      <span>{{ $t("files.lonely") }}</span>
      </h2>
    </div>
  </div>
</template>

<script>
import { accessApi } from "@/api";
import { state, mutations } from "@/store";
import Errors from "@/views/Errors.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "accessSettings",
  components: {
    Errors,
  },
  data: function () {
    return {
      rules: {},
      accessPath: "",
      error: null,
      selectedSource: "",
      loading: false,
    };
  },
  async mounted() {
    this.selectedSource = state.sources.current;
    await this.fetchRules();
    // Listen for access rule changes
    eventBus.on('accessRulesChanged', this.fetchRules);
  },
  beforeUnmount() {
    // Clean up event listener
    eventBus.off('accessRulesChanged', this.fetchRules);
  },
  computed: {
    /*loading() {
      return state.loading;
    },*/
    availableSources() {
      return Object.keys(state.sources.info);
    },
  },
  methods: {
    async fetchRules() {
      this.loading = true;
      this.error = null;
      this.accessPath = state.req.path || '/';
      try {
        this.rules = await accessApi.getAll(this.selectedSource);
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
      }
    },
    addAccess() {
      mutations.showHover({
        name: "access",
        props: {
          sourceName: this.selectedSource,
          path: "/"
        }
      });
    },
    editAccess(path) {
      mutations.showHover({
        name: "access",
        props: {
          sourceName: this.selectedSource,
          path: path
        }
      });
    },
  },
};
</script>
