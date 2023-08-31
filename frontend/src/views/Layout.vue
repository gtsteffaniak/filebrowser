<template>
  <div>
    <div v-if="progress" class="progress">
      <div v-bind:style="{ width: this.progress + '%' }"></div>
    </div>
    <editorBar v-if="getCurrentView === 'editor'"></editorBar>
    <listingBar v-else-if="getCurrentView === 'listing'"></listingBar>
    <previewBar v-else-if="getCurrentView === 'preview'"></previewBar>
    <defaultBar v-else></defaultBar>
    <sidebar></sidebar>
    <main>
      <router-view></router-view>
      <shell v-if="isExecEnabled && isLogged && user.perm.execute" />
    </main>
    <prompts></prompts>
    <upload-files></upload-files>
  </div>
</template>

<script>
import editorBar from "./files/Editor.vue"
import defaultBar from "./files/Default.vue"
import listingBar from"./files/Listing.vue"
import previewBar from "./files/Preview.vue"
import Action from "@/components/header/Action";
import { mapState, mapGetters } from "vuex";
import Sidebar from "@/components/Sidebar";
import Prompts from "@/components/prompts/Prompts";
import Shell from "@/components/Shell";
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
    Shell,
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
    ...mapGetters(["isLogged", "progress"]),
    ...mapState(["req", "user", "currentView"]),

    isExecEnabled: () => enableExec,
    getCurrentView() {
      return this.$store.state.currentView;
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
      if (this.$route.path.startsWith('/settings/')){
        title = "Settings"
      }
      return title
    },
  },
};
</script>
