<template>
  <nav :class="{ active }">
    <template v-if="isLogged">
      <button class="action" @click="toRoot" :aria-label="$t('sidebar.myFiles')" :title="$t('sidebar.myFiles')">
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.myFiles") }}</span>
      </button>
      <div v-if="user.perm.create">
        <button @click="$store.commit('showHover', 'newDir')" class="action" :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')">
          <i class="material-icons">create_new_folder</i>
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>
        <button @click="$store.commit('showHover', 'newFile')" class="action" :aria-label="$t('sidebar.newFile')"
          :title="$t('sidebar.newFile')">
          <i class="material-icons">note_add</i>
          <span>{{ $t("sidebar.newFile") }}</span>
        </button>
        <button id="upload-button" @click="upload($event)" class="action" :aria-label="$t('sidebar.upload')" >
          <i class="material-icons">file_upload</i>
          <span>Upload file</span>
        </button>
      </div>
      <div>
        <button class="action" @click="toSettings" :aria-label="$t('sidebar.settings')" :title="$t('sidebar.settings')">
          <i class="material-icons">settings_applications</i>
          <span>{{ $t("sidebar.settings") }}</span>
        </button>

        <button v-if="canLogout" @click="logout" class="action" id="logout" :aria-label="$t('sidebar.logout')"
          :title="$t('sidebar.logout')">
          <i class="material-icons">exit_to_app</i>
          <span>{{ $t("sidebar.logout") }}</span>
        </button>
      </div>
    </template>
    <template v-else>
      <router-link class="action" to="/login" :aria-label="$t('sidebar.login')" :title="$t('sidebar.login')">
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.login") }}</span>
      </router-link>
      <router-link v-if="signup" class="action" to="/login" :aria-label="$t('sidebar.signup')"
        :title="$t('sidebar.signup')">
        <i class="material-icons">person_add</i>
        <span>{{ $t("sidebar.signup") }}</span>
      </router-link>
    </template>
    <div class="credits" v-if="$router.currentRoute.path.includes('/files/') && !disableUsedPercentage
      ">
      <progress-bar :val="usage.usedPercentage" size="medium"></progress-bar>
      <span style="text-align:center">{{ usage.usedPercentage }}%</span>
      <span>{{ usage.used }} of {{ usage.total }} used</span>
      <br>
      <span v-if="disableExternal">File Browser</span>
      <span v-else>
        <a rel="noopener noreferrer" target="_blank" href="https://github.com/gtsteffaniak/filebrowser">
          File Browser
        </a>
      </span>
      <span>{{ version }}</span>
      <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span>
    </div>
  </nav>
</template>
<script>
import { mapState, mapGetters } from "vuex";
import * as upload from "@/utils/upload";
import * as auth from "@/utils/auth";
import {
  version,
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { files as api } from "@/api";
import ProgressBar from "vue-simple-progress";
import prettyBytes from "pretty-bytes";

export default {
  name: "sidebar",
  components: {
    ProgressBar,
  },
  computed: {
    ...mapState(["user"]),
    ...mapGetters(["isLogged"]),
    active() {
      return this.$store.state.show === "sidebar";
    },
    signup: () => signup,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
  },
  asyncComputed: {
    usage: {
      async get() {
        let path = this.$route.path.endsWith("/")
          ? this.$route.path
          : this.$route.path + "/";
        let usageStats = { used: 0, total: 0, usedPercentage: 0 };
        if (this.disableUsedPercentage) {
          return usageStats;
        }
        try {
          let usage = await api.usage(path);
          usageStats = {
            used: prettyBytes(usage.used / 1024, { binary: true }),
            total: prettyBytes(usage.total / 1024, { binary: true }),
            usedPercentage: Math.round((usage.used / usage.total) * 100),
          };
        } catch (error) {
          this.$showError(error);
        }
        return usageStats;
      },
      default: { used: "0 B", total: "0 B", usedPercentage: 0 },
      shouldUpdate() {
        return this.$router.currentRoute.path.includes("/files/");
      },
    },
  },
  methods: {
    toRoot() {
      this.$router.push({ path: "/files/" }, () => { });
      this.$store.commit("closeHovers");
    },
    toSettings() {
      this.$router.push({ path: "/settings" }, () => { });
      this.$store.commit("closeHovers");
    },
    help() {
      this.$store.commit("showHover", "help");
    },
    upload: function () {
      if (
        typeof window.DataTransferItem !== "undefined" &&
        typeof DataTransferItem.prototype.webkitGetAsEntry !== "undefined"
      ) {
        this.$store.commit("showHover", "upload");
      } else {
        document.getElementById("upload-input").click();
      }
    },
    uploadInput(event) {
      this.$store.commit("closeHovers");

      let files = event.currentTarget.files;
      let folder_upload =
        files[0].webkitRelativePath !== undefined && files[0].webkitRelativePath !== "";

      if (folder_upload) {
        for (let i = 0; i < files.length; i++) {
          let file = files[i];
          files[i].fullPath = file.webkitRelativePath;
        }
      }

      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let conflict = upload.checkConflict(files, this.req.items);

      if (conflict) {
        this.$store.commit("showHover", {
          prompt: "replace",
          confirm: (event) => {
            event.preventDefault();
            this.$store.commit("closeHovers");
            upload.handleFiles(files, path, true);
          },
        });

        return;
      }

      upload.handleFiles(files, path);
    },
    logout: auth.logout,
  },
};
</script>
