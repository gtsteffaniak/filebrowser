<template>
  <!-- Conditionally render the DocumentEditor component -->
    <DocumentEditor v-if="ready"
      id="docEditor" 
      :documentServerUrl="openofficeurl" 
      :config="config"
      :events_onDocumentReady="onDocumentReady" 
      :onLoadComponentError="onLoadComponentError" 
    />
  <div v-else>
    <p>Loading editor...</p>
  </div>
</template>

<script lang="ts">
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { onlyOfficeUrl } from "@/utils/constants";
import { state } from "@/store";
import { filesApi } from "@/api";
import cookie from "@/utils/cookie";

export default {
  name: "onlyOfficeEditor",
  components: {
    DocumentEditor,
  },
  data() {
    return {
      ready: false, // Flag to indicate whether the setup is complete
      config: {
        document: {
          fileType: "docx",
          permissions: {
            chat: false,
            edit: false,
            review: false,
            fillforms: false,
            comment: false,
          },
        },
        editorConfig: {
          callbackUrl: "api/onlyoffice/callback",
          customization: {
            autosave: true,
            forcesave: true,
            uiTheme: "default-dark",
          },
          lang: "en",
          mode: "edit",
          user: {
            id: "1",
            name: "admin",
          },
        },
        type: "desktop",
        documentType: "word",
      },
    };
  },
  computed: {
    req() {
      return state.req;
    },
    openofficeurl() {
      return onlyOfficeUrl;
    },
  },
  async mounted() {
    // Perform the setup and update the config
    try {
      this.config.document.url = await filesApi.getDownloadURL(state.req.path,false,state.jwt);
      this.config.document.fileType = state.req.name.split(".").pop(); // Fix fileType extraction
      this.config.document.key = state.req.onlyOfficeId;
      this.config.document.title = state.req.name;
      this.config.editorConfig.user.id = state.user.id;
      this.config.editorConfig.user.name = state.user.username;
      this.config.type = state.isMobile ? "mobile" : "desktop";
      this.config.token = state.req.onlyOfficeSecret;
      console.log(this.config);
      // Mark as ready to render the component
      this.ready = true;
    } catch (error) {
      console.error("Error during setup:", error);
      // Handle setup failure if needed
    }
  },
  methods: {
    onDocumentReady() {
      console.log("Document is loaded");
    },
    onLoadComponentError(errorCode, errorDescription) {
      switch (errorCode) {
        case -1: // Unknown error loading component
          console.log(errorDescription);
          break;

        case -2: // Error load DocsAPI from http://documentserver/
          console.log(errorDescription);
          break;

        case -3: // DocsAPI is not defined
          console.log(errorDescription);
          break;
      }
    },
    getCookie(name) {
      let cookie = document.cookie
        .split(";")
        .find((cookie) => cookie.includes(name + "="));
      if (cookie != null) {
        return cookie.split("=")[1];
      }
      return ""
    },
  },
};
</script>
