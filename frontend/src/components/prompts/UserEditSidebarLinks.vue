<template>
  <div class="card-content prompt-panel">
    <SidebarLinksEditor
      embedded
      mode="user"
      :initial-sidebar-links="sidebarLinks"
      :initial-show-tools-in-sidebar="showToolsInSidebar"
      :show-prompt-actions="true"
      @save="saveLinks"
    />
  </div>
</template>

<script>
import { mutations } from "@/store";
import SidebarLinksEditor from "@/components/sidebar/SidebarLinksEditor.vue";
import {
  getUserEditSession,
  updateUserEditSession,
} from "@/utils/userEditSession";

export default {
  name: "user-edit-sidebar-links",
  components: {
    SidebarLinksEditor,
  },
  computed: {
    session() {
      return getUserEditSession();
    },
    sidebarLinks() {
      return Array.isArray(this.session?.user?.sidebarLinks)
        ? this.session.user.sidebarLinks
        : [];
    },
    showToolsInSidebar() {
      const value = this.session?.user?.showToolsInSidebar;
      return typeof value === "boolean" ? value : true;
    },
  },
  mounted() {
    void mutations.syncSidebarLinkDefaultsPolicy();
  },
  methods: {
    saveLinks({ links, showToolsInSidebar }) {
      if (!this.session?.user) {
        return;
      }
      updateUserEditSession({
        user: {
          ...this.session.user,
          sidebarLinks: [...links],
          showToolsInSidebar,
        },
      });
      mutations.closeTopPrompt();
    },
  },
};
</script>
