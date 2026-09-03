<template>
  <SidebarLinksEditor
    ref="editor"
    :mode="context"
    :share-data="shareData"
    :show-prompt-actions="true"
    @save="saveLinks"
  />
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { shareApi } from "@/api";
import { eventBus } from "@/store/eventBus";
import SidebarLinksEditor from "@/components/sidebar/SidebarLinksEditor.vue";

export default {
  name: "SidebarLinks",
  components: {
    SidebarLinksEditor,
  },
  props: {
    context: {
      type: String,
      default: "user",
    },
    shareData: {
      type: Object,
      default: null,
    },
  },
  methods: {
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
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>
