<template>
  <div>
    <div v-if="progress" class="progress">
      <div v-bind:style="{ width: this.progress + '%' }"></div>
    </div>
    <defaultBar v-if="currentView === 'listing'"></defaultBar>
    <editorBar v-else-if="currentView === 'editor'"></editorBar>
    <editorBar v-else-if="currentView === 'share'"></editorBar>
    <editorBar v-else-if="currentView === 'dashboard'"></editorBar>
    <editorBar v-else-if="currentView === 'error'"></editorBar>
    <defaultBar v-else></defaultBar>
    <sidebar></sidebar>
    <main>
      <router-view></router-view>
    </main>
    <prompts></prompts>
    <upload-files></upload-files>
  </div>
</template>

<script>
import editorBar from "./files/EditorBar.vue"
import defaultBar from "./files/Default.vue"
import listingBar from "./files/Listing.vue"
import previewBar from "./files/Preview.vue"
import Prompts from "@/components/prompts/Prompts";
import Action from "@/components/header/Action";
import { mapState, mapGetters } from "vuex";
import Sidebar from "@/components/Sidebar.vue";
import UploadFiles from "../components/prompts/UploadFiles";
import { enableExec } from "@/utils/constants";
export default {
  name: "layout",
  components: {
    defaultBar,
    editorBar,
    listingBar,
    previewBar,
    Action,
    Sidebar,
    Prompts,
    UploadFiles,
  },
  data: function () {
    return {
      showContexts: true,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
    };
  },
  computed: {
    ...mapGetters(["isLogged", "progress", "isListing"]),
    ...mapState(["req", "user", "state"]),

    isExecEnabled: () => enableExec,
    currentView() {
      if (this.req.type == undefined) {
        return null;
      }

      if (this.req.isDir) {
        return "listing";
      } else if (
        this.req.type === "text" ||
        this.req.type === "textImmutable"
      ) {
        return "editor";
      } else {
        return "preview";
      }
    },
  },
  watch: {
    $route: function () {
      this.$store.commit("resetSelected");
      this.$store.commit("multiple", false);
      if (this.$store.state.show !== "success") this.$store.commit("closeHovers");
    },
  },
  methods: {
    getTitle() {
      let title = "Title"
      if (this.$route.path.startsWith('/settings/')) {
        title = "Settings"
      }
      return title
    },
  },
};
</script>