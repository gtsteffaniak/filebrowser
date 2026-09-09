<template>
  <ToolsAccessEditor
    embedded
    mode="user"
    :model-value="policyItems"
    :user-tool-access="toolAccess"
    :show-prompt-actions="true"
    @update:user-tool-access="onToolAccessChange"
    @save="save"
    @cancel="closeTopPrompt"
  />
</template>

<script>
import { mutations, state } from "@/store";
import { getToolAccessDefaults } from "@/api/settings";
import ToolsAccessEditor from "@/components/tools/ToolsAccessEditor.vue";
import {
  getUserEditSession,
  updateUserEditSession,
} from "@/utils/userEditSession";
import { normalizeUserToolAccess } from "@/utils/toolAccess";

export default {
  name: "user-edit-tools",
  components: {
    ToolsAccessEditor,
  },
  data() {
    return {
      defaultItems: [],
    };
  },
  computed: {
    session() {
      return getUserEditSession();
    },
    toolAccess() {
      const raw = this.session?.user?.toolAccess || {};
      return normalizeUserToolAccess(raw, this.policyItems);
    },
    policyItems() {
      if (this.defaultItems.length > 0) {
        return this.defaultItems;
      }
      return state.toolAccessDefaultsPolicy?.items || [];
    },
  },
  async mounted() {
    await mutations.syncToolAccessDefaultsPolicy();
    try {
      const data = await getToolAccessDefaults();
      this.defaultItems = Array.isArray(data?.items) ? data.items : [];
    } catch {
      this.defaultItems = [];
    }
  },
  methods: {
    onToolAccessChange(toolAccess) {
      if (!this.session?.user) {
        return;
      }
      updateUserEditSession({
        user: {
          ...this.session.user,
          toolAccess: normalizeUserToolAccess(toolAccess, this.policyItems),
        },
      });
    },
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    save() {
      this.closeTopPrompt();
    },
  },
};
</script>
