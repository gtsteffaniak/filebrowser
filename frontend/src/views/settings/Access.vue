<template>
  <errors v-if="error" :errorCode="error.status" />

  <div class="card-title">
    <h2>{{ $t("access.accessManagement") }}</h2>
  </div>
  <div class="card-content full" v-if="Object.keys(rules).length > 0">
    <table aria-label="Access Rules">
      <thead>
        <tr>
          <th>path</th>
          <th>denied users</th>
          <th>denied groups</th>
          <th>allowed users</th>
          <th>allowed groups</th>
          <th>actions</th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="(rule, path) in rules" :key="path">
          <td>{{ path }}</td>
          <td>{{ rule.deny.users.length }}</td>
          <td>{{ rule.deny.groups.length }}</td>
          <td>{{ rule.allow.users.length }}</td>
          <td>{{ rule.allow.groups.length }}</td>
          <td class="small">
            <button class="action" @click="editAccess(path)" :aria-label="$t('buttons.edit')"
              :title="$t('buttons.edit')">
              <i class="material-icons">edit</i>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <h2 class="message" v-else>
    <i class="material-icons">sentiment_dissatisfied</i>
    <span>{{ $t("files.lonely") }}</span>
  </h2>

</template>

<script>
import { notify } from "@/notify";
import { accessApi } from "@/api";
import { state, mutations, getters } from "@/store";
import Clipboard from "clipboard";
import Errors from "@/views/Errors.vue";

export default {
  name: "accessSettings",
  components: {
    Errors,
  },
  data: function () {
    return {
      rules: {},
      accessPath: ""
    };
  },
  async mounted() {
    this.accessPath = state.req.path;
    this.rules = await accessApi.getAll(state.sources.current);
    console.log(this.rules);
  },
  computed: {
  },
  methods: {
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
