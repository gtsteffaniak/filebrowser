<template>
  <!-- Conditionally render the DocumentEditor component -->
  <DocumentEditor
    v-if="ready"
    id="docEditor"
    :documentServerUrl="onlyOfficeUrl"
    :config="clientConfig"
    :onLoadComponentError="onLoadComponentError"
  />
  <div v-else>
    <p>{{ $t('files.loading') }}</p>
  </div>
</template>

<script lang="ts">
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { onlyOfficeUrl } from "@/utils/constants";
import { state } from "@/store";
import { fetchJSON } from "@/api/utils";
import { filesApi } from "@/api";
import { baseURL } from "@/utils/constants";

export default {
  name: "onlyOfficeEditor",
  components: {
    DocumentEditor,
  },
  data() {
    return {
      ready: false, // Flag to indicate whether the setup is complete
      clientConfig: {},
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
      const refUrl = await filesApi.getDownloadURL(
        state.req.source,
        state.req.path,
        false,
        true
      );
      let configData = await fetchJSON(baseURL + `api/onlyoffice/config?url=${refUrl}`);
      configData.type = state.isMobile ? "mobile" : "desktop";
      this.clientConfig = configData;
      console.log("Client config:", this.clientConfig);
      this.ready = true;
    } catch (error) {
      console.error("Error during setup:", error);
      // Handle setup failure if needed
    }
  },
};
</script>
