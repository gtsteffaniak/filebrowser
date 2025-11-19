<template>
  <!-- Render all prompts in the hover stack, but only show the active one -->
  <div
    v-for="(prompt, index) in prompts"
    :key="'prompt-' + index + '-' + prompt.name"
    class="card floating fb-shadow"
    :class="{ 'dark-mode': isDarkMode }"
    v-show="index === prompts.length - 1"
    :aria-label="prompt.name + '-prompt'"
  >
    <component
      :ref="prompt.name"
      :is="prompt.name"
      v-bind="getPropsForPrompt(prompt)"
    />
  </div>
</template>

<script>
import Help from "./Help.vue";
import Info from "./Info.vue";
import Delete from "./Delete.vue";
import Rename from "./Rename.vue";
import Download from "./Download.vue";
import MoveCopy from "./MoveCopy.vue";
import NewFile from "./NewFile.vue";
import NewDir from "./NewDir.vue";
import Replace from "./Replace.vue";
import ReplaceRename from "./ReplaceRename.vue";
import Share from "./Share.vue";
import Upload from "./Upload.vue";
import ShareDelete from "./ShareDelete.vue";
import DeleteUser from "./DeleteUser.vue";
import CreateApi from "./CreateApi.vue";
import ActionApi from "./ActionApi.vue";
import SidebarLinks from "./SidebarLinks.vue";
import IconPicker from "./IconPicker.vue";
import Sidebar from "../sidebar/Sidebar.vue";
import UserEdit from "./UserEdit.vue";
import buttons from "@/utils/buttons";
import Totp from "./Totp.vue";
import Access from "./Access.vue";
import Password from "./Password.vue";
import PlaybackQueue from "./PlaybackQueue.vue";
import FileList from "./FileList.vue";
import PathPicker from "./PathPicker.vue";
import SaveBeforeExit from "./SaveBeforeExit.vue";
import CopyPasteConfirm from "./CopyPasteConfirm.vue";
import CloseWithActiveUploads from "./CloseWithActiveUploads.vue";
import Generic from "./Generic.vue";
import ShareInfo from "./ShareInfo.vue";
import { state, getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "prompts",
  components: {
    UserEdit,
    Info,
    Delete,
    Rename,
    Download,
    Move: MoveCopy,
    Copy: MoveCopy,
    Share,
    NewFile,
    NewDir,
    Help,
    Replace,
    ReplaceRename,
    Totp,
    Upload,
    ShareDelete,
    Sidebar,
    DeleteUser,
    CreateApi,
    ActionApi,
    SidebarLinks,
    IconPicker,
    Access,
    Password,
    PlaybackQueue,
    "file-list": FileList,
    PathPicker,
    SaveBeforeExit,
    CopyPasteConfirm,
    CloseWithActiveUploads,
    generic: Generic,
    ShareInfo,
  },
  data() {
    return {
      pluginData: {
        buttons,
        store: state, // Directly use state
        router: this.$router,
      },
    };
  },
  created() {
    window.addEventListener("keydown", (event) => {
      let currentPrompt = getters.currentPrompt();
      if (!currentPrompt) return;

      let prompt = this.$refs[currentPrompt.name];
      // Handle array refs (Vue 3 style)
      if (Array.isArray(prompt)) {
        prompt = prompt[0];
      }

      // Esc!
      if (event.keyCode === 27) {
        event.stopImmediatePropagation();
        mutations.closeHovers();
      } else if (event.KeyCode === 8) {
        event.preventDefault();
      }
      // Enter
      if (event.keyCode === 13 && prompt) {
        switch (currentPrompt.name) {
          case "delete":
            prompt.submit();
            break;
          case "copy":
          case "move":
            prompt.performOperation(event);
            break;
          case "replace":
            prompt.showConfirm(event);
            break;
          case "generic":
            prompt.submit();
            break;
          case "CopyPasteConfirm":
            prompt.confirm();
            break;
        }
      }
    });
  },
  computed: {
    prompts() {
      return state.prompts || [];
    },
    plugins() {
      return state.plugins;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
  methods: {
    getPropsForPrompt(prompt) {
      // For move and copy prompts, add the operation prop
      if (prompt.name === "move" || prompt.name === "copy") {
        return {
          ...prompt.props,
          operation: prompt.name,
        };
      }
      return prompt.props || {};
    },
  },
};
</script>
