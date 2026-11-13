<template>
  <SidebarLinksEditor
    ref="editor"
    embedded
    :mode="context"
    :share-data="shareData"
    :show-prompt-actions="true"
    @save="saveLinks"
    @cancel="closeTopPrompt"
  />
</template>

<script>
import { notify } from "@/notify/index.ts";
import { mutations } from "@/store/index.ts";
import { shareApi } from "@/api/index.js";
import { eventBus } from "@/store/eventBus.ts";
import SidebarLinksEditor from "@/components/sidebar/SidebarLinksEditor.vue";

export default {
  name: "SidebarLinks",
  components: {
    SidebarLinksEditor,
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
    context: {
      type: String,
      default: "user",
    },
    shareData: {
      type: Object,
      default: null,
    },
  },
  mounted() {
    if (this.context === "user") {
      void mutations.syncSidebarLinkDefaultsPolicy();
    }
  },
  methods: {
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    async saveLinks({ links, showToolsInSidebar }) {
      try {
        if (this.context === "share") {
          const payload = {
            hash: this.shareData.hash,
            sidebarLinks: links,
          };

          await shareApi.create(payload);

          eventBus.emit("shareSidebarLinksUpdated", {
            hash: this.shareData.hash,
            sidebarLinks: links,
          });
        } else {
          await mutations.updateCurrentUser({
            sidebarLinks: [...links],
            showToolsInSidebar,
          });
        }

        mutations.closeTopPrompt();
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>
