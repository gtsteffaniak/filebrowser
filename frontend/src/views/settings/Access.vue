<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("access.accessManagement") }}</h2>
    <button class="button" @click="addAccess">{{ $t("buttons.new") }}</button>
  </div>
  <div class="card-content full" >

    <table aria-label="Access Rules">
      <thead>
        <tr>
          <th>{{$t('settings.path')}}</th>
          <th>{{$t('access.totalDenied')}}</th>
          <th>{{$t('access.totalAllowed')}}</th>
          <th>{{$t('buttons.edit') }}</th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="(rule, path) in rules" :key="path">
          <td>{{ path }}</td>
          <td>{{ rule.deny.users.length + rule.deny.groups.length }}</td>
          <td>{{ rule.allow.users.length + rule.allow.groups.length }}</td>
          <td class="small">
            <button class="action" @click="editAccess(path)" :aria-label="$t('buttons.edit')"
              :title="$t('buttons.edit')">
              <i class="material-icons">edit</i>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="Object.keys(rules).length === 0">
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

export default {
  name: "accessSettings",
  components: {
    Errors,
  },
  data: function () {
    return {
      rules: {
        // Initialize rules with a structure that includes deny and allow properties
        // This helps the linter understand the shape of the `rule` object in the template.
        // The actual data will be populated by the API call.
        somePath: { // This is just an example key to define the structure
          deny: { users: [], groups: [] },
          allow: { users: [], groups: [] },
        },
      },
      accessPath: "",
      error: null,
    };
  },
  async mounted() {
    this.accessPath = state.req.path || '/';
    try {
      this.rules = await accessApi.getAll(state.sources.current);
    } catch (e) {
      this.error = e;
    }
    console.log(this.rules);
  },
  computed: {
  },
  methods: {
    addAccess() {
      mutations.showHover({
        name: "access",
        props: {
          sourceName: state.sources.current,
          path: "/"
        }
      });
    },
    editAccess(path) {
      mutations.showHover({
        name: "access",
        props: {
          sourceName: state.sources.current,
          path: path
        }
      });
    },
  },
};
</script>
