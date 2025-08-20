<template>
  <!-- Conditionally render the DocumentEditor component -->
  <DocumentEditor v-if="ready" id="docEditor" :documentServerUrl="onlyOfficeUrl" :config="clientConfig"
    :onLoadComponentError="onLoadComponentError" />
  <div v-else>
    <p>{{ $t("files.loading") }}</p>
  </div>
  <div @click="close" class="floating-close button" :class="{ 'float-in': floatIn }">
    <i class="material-icons">close</i>
  </div>
</template>

<script lang="ts">
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { onlyOfficeUrl } from "@/utils/constants";
import { state, getters } from "@/store";
import { fetchJSON } from "@/api/utils";
import { filesApi, publicApi } from "@/api";
import { baseURL } from "@/utils/constants";
import { removeLastDir } from "@/utils/url";

export default {
  name: "onlyOfficeEditor",
  components: {
    DocumentEditor,
  },
  data() {
    return {
      ready: false, // Flag to indicate whether the setup is complete
      clientConfig: {},
      floatIn: false, // Flag for the float-in animation
    };
  },
  computed: {
    req() {
      return state.req;
    },
    onlyOfficeUrl() {
      return onlyOfficeUrl;
    },
  },
  async mounted() {
    // Perform the setup and update the config
    try {
      const refUrl = getters.isShare()
        ? publicApi.getDownloadURL({
          path: state.share.subPath,
          hash: state.share.hash,
          token: state.share.token,
        }, [state.req.path])
        : await filesApi.getDownloadURL(
          state.req.source,
          state.req.path,
          false,
          true
        );
      let configData;
      let configUrl = `api/onlyoffice/config?url=${encodeURIComponent(refUrl)}`;
      if (getters.isShare()) {
        configUrl = configUrl + `&hash=${state.share.hash}`;
      }
      configData = await fetchJSON(baseURL + configUrl);

      configData.type = state.isMobile ? "mobile" : "desktop";
      this.clientConfig = configData;
      console.log("Client config:", this.clientConfig);
      this.ready = true;
      // Trigger float-in animation
      setTimeout(() => {
        this.floatIn = true;
      }, 100); // slight delay to allow rendering
    } catch (error) {
      console.error("Error during setup:", error);
      // Handle setup failure if needed
    }
  },
  methods: {
    close() {
      const current = window.location.pathname;
      const newpath = removeLastDir(current);
      window.location = newpath + "#" + state.req.name;
    },
  },
};
</script>

<style scoped>
.floating-close {
  position: fixed;
  left: 50%;
  transform: translate(-50%, -5em);
  /* Start offscreen */
  transition: transform 0.4s ease;
  background: var(--surfaceSecondary);
  font-size: .5em;
  top: 0;
}

.float-in {
  transform: translate(-50%, 2.75em);
  /* Animate to final position */
}

.floating-close i {
  font-size: 2em;
  padding-right: 1em;
  padding-left: 1em;
}
</style>
