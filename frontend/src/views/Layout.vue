<template>
  <div>
    <div v-if="progress" class="progress">
      <div v-bind:style="{ width: this.progress + '%' }"></div>
    </div>
    <header-bar showMenu showLogo>
      <search />
      <template #actions>
        <action icon="grid_view" :label="$t('buttons.switchView')" @action="switchView" />
      </template>
    </header-bar>
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
import { mapState, mapGetters } from "vuex";
import Sidebar from "@/components/Sidebar";
import Prompts from "@/components/prompts/Prompts";
import Shell from "@/components/Shell";
import UploadFiles from "../components/prompts/UploadFiles";
import { enableExec } from "@/utils/constants";
import HeaderBar from "@/components/header/HeaderBar";
import Search from "@/components/Search";
import Action from "@/components/header/Action";

export default {
  name: "layout",
  components: {
    Action,
    HeaderBar,
    Search,
    Sidebar,
    Prompts,
    Shell,
    UploadFiles,
  },
  computed: {
    ...mapGetters(["isLogged", "progress"]),
    ...mapState(["user"]),
    isExecEnabled: () => enableExec,
  },
  watch: {
    $route: function () {
      this.$store.commit("resetSelected");
      this.$store.commit("multiple", false);
      if (this.$store.state.show !== "success")
        this.$store.commit("closeHovers");
    },
  },
  methods: {
    switchView: async function () {
      this.$store.commit("closeHovers");
      const modes = {
        list: "mosaic",
        mosaic: "mosaic gallery",
        "mosaic gallery": "list",
      };

      const data = {
        id: this.user.id,
        viewMode: modes[this.user.viewMode] || "list",
      };
      //users.update(data, ["viewMode"]).catch(this.$showError);
      this.$store.commit("updateUser", data);

      //this.setItemWeight();
      //this.fillWindow();
    },
  }

};
</script>
