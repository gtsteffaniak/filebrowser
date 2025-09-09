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

<script>
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { onlyOfficeUrl } from "@/utils/constants";
import { state, getters, mutations } from "@/store";
import { baseURL } from "@/utils/constants";
import { removeLastDir } from "@/utils/url";
import { filesApi } from "@/api";

const wikiLink = "https://github.com/gtsteffaniak/filebrowser/wiki/Office-Support#onlyoffice-integration-troubleshooting-guide"
const wikiLinkText = "📖 View Troubleshooting Guide"
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
      source: "",
      path: "",
      debugInfo: "",
      debugMode: false,
      hasErrors: false,
      onlyOfficeServerCheck: "pending",
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
    this.source = state.req.source;
    this.path = state.req.path;

    // Expose closeTooltip method to global window for tooltip button access
    window.closeTooltip = () => this.closeTooltip();

    // Initialize debug mode if enabled
    if (state.user.debugOffice) {
      this.debugMode = true;
      this.showDebugInfo();
    }

    // Perform the setup and update the config with simplified API parameters
    try {
      // Update debug info
      if (this.debugMode) {
        this.updateDebugStatus("✅ Configuration Check - URL Built");
      }

      // Fetch configuration from backend
      const configData = await filesApi.GetOfficeConfig(state.req)

      if (this.debugMode) {
        this.updateDebugStatus("✅ API Request - Config Received");
      }

      configData.type = state.isMobile ? "mobile" : "desktop";
      this.clientConfig = configData;
      console.log("OnlyOffice client config received:", this.clientConfig);

      if (this.debugMode) {
        this.updateDebugStatus("✅ OnlyOffice Server Connection - Config Sent");
      }

      this.ready = true;

      // Trigger float-in animation
      setTimeout(() => {
        this.floatIn = true;

        // Monitor for successful initialization and check OnlyOffice server
        if (this.debugMode) {
          this.checkOnlyOfficeServer();
        }
      }, 100); // slight delay to allow rendering

    } catch (error) {
      console.error("Error during OnlyOffice setup:", error);

      if (this.debugMode) {
        const errorMsg = (error && typeof error === 'object' && 'message' in error) ? error.message : String(error);
        this.updateDebugStatus(`❌ Setup Error: ${errorMsg}<br/><a href="${wikiLink}" target="_blank">${wikiLinkText}</a>`);
      }
      // TODO: Show user-friendly error message
    }
  },
  methods: {
    getInternalUrlInfo() {
      if (this.clientConfig && this.clientConfig.document && this.clientConfig.document.url) {
        try {
          const docUrlOrigin = new URL(this.clientConfig.document.url).origin;
          const windowOrigin = window.location.origin;

          if (docUrlOrigin !== windowOrigin) {
            return {
              isSet: true,
              message: `${docUrlOrigin}`
            };
          } else {
            return {
              isSet: false,
              message: "Not set, using window.location"
            };
          }
        } catch (e) {
          return {
            isSet: false,
            message: "⚠️ Error parsing document URL"
          };
        }
      }
      return {
        isSet: false,
        message: "Analyzing..."
      };
    },

    close() {
      const current = window.location.pathname;
      const newpath = removeLastDir(current);
      // Get filename from path
      const filename = this.path.split('/').pop() || "";
      window.location.href = newpath + "#" + filename;
    },

    closeTooltip() {
      mutations.hideTooltip();
    },

    showDebugInfo() {
      if (!this.debugMode) return;

      // Just initialize with the first step - no full HTML structure here
      this.debugInfo = "⏳ Starting OnlyOffice initialization...";

      // Define variables needed for template
      const filename = this.path.split('/').pop() || "";
      const isShare = getters.isShare();

      // Build and show the initial tooltip with enhanced properties
      let content = `
        <div style="font-family: monospace; font-size: 12px; line-height: 1.4;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
            <h3 style="margin: 0; color: #2196F3;">🔧 OnlyOffice Debug Trace</h3>
            <button class="button tooltip-close-button" onclick="window.closeTooltip()" >x</button>
          </div>

          <div style="margin-bottom: 15px; padding: 10px; background: #f5f5f5; border-radius: 4px;">
            <strong>Configuration:</strong><br/>
            OnlyOffice URL: ${this.onlyOfficeUrl}<br/>
            Internal URL: ${this.getInternalUrlInfo().message}<br/>
            Base URL: ${baseURL}<br/>
            Source: ${this.source}<br/>
            Path: ${this.path}<br/>
            Filename: ${filename}<br/>
            ${isShare ? `Share Hash: ${state.share.hash}` : 'User Request'}<br/>
          </div>

          <div style="margin-bottom: 15px;">
            <strong>Process Steps:</strong><br/>
            ${this.debugInfo}
          </div>
        </div>
      `;

      state.tooltip.content = content;
      state.tooltip.show = true;
      state.tooltip.pointerEvents = true; // Enable clicks for links
      state.tooltip.width = "50vw"; // Wider for debug info
      state.tooltip.x = 50;
      state.tooltip.y = 50;
    },

    updateDebugStatus(message) {
      if (!this.debugMode) return;

      // Append new message to debug info
      this.debugInfo = this.debugInfo + "<br/>" + message;

      // Update the tooltip
      this.updateDebugTooltip();
    },

    updateDebugTooltip() {
      if (!this.debugMode) return;

      const filename = this.path.split('/').pop() || "";
      const isShare = getters.isShare();
      const internalUrlInfo = this.getInternalUrlInfo();

      // Build config details if available
      let configDetailsHtml = "";
      if (this.clientConfig && this.clientConfig.document) {
        const doc = this.clientConfig.document;
        const editor = this.clientConfig.editorConfig;

        // Extract domains for network flow analysis
        const downloadDomain = doc.url ? new URL(doc.url).origin : 'N/A';

        configDetailsHtml = `
          <div style="margin-bottom: 15px; padding: 10px; background: #e3f2fd; border-radius: 4px;">
            <strong>OnlyOffice Configuration Details:</strong><br/>
            Document Key: ${doc.key}<br/>
            File Type: ${doc.fileType}<br/>
            Edit Mode: ${editor ? editor.mode : 'N/A'}<br/>
            Download URL: ${doc.url ? doc.url.substring(0, 80) + '...' : 'N/A'}<br/>
            Callback URL: ${editor && editor.callbackUrl ? editor.callbackUrl.substring(0, 80) + '...' : 'N/A'}<br/>
            <br/>
            <strong>Network Flow:</strong><br/>
            Browser (${window.location.origin}) ↔ OnlyOffice: ${this.onlyOfficeUrl}<br/>
            OnlyOffice → FileBrowser: ${downloadDomain}<br/>
          </div>
        `;
      }

      // Determine overall status
      let overallStatus = "";
      if (this.hasErrors) {
        overallStatus = `
          <div style="margin-top: 15px; padding: 10px; background: #ffebee; border-radius: 4px; color: #c62828;">
            <strong>❌ Error Detected</strong><br/>
            OnlyOffice integration failed. Check the failed steps above.<br/>
            <a href="${wikiLink}" target="_blank" style="color: #1976d2;">
              ${wikiLinkText}
            </a>
          </div>
        `;
      } else if (this.debugInfo.includes("🎉") && !this.hasErrors) {
        overallStatus = `
          <div style="margin-top: 15px; padding: 10px; background: #e8f5e8; border-radius: 4px; color: #2e7d32;">
            <strong>🎉 Success!</strong><br/>
            OnlyOffice integration completed successfully. No issues found!
          </div>
        `;
      }

      let content = `
        <div style="font-family: monospace; font-size: 11px; line-height: 1.4;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
            <h3 style="margin: 0; color: #2196F3;">🔧 OnlyOffice Debug Trace</h3>
            <button class="button tooltip-close-button" onclick="window.closeTooltip()" >x</button>
          </div>

          <div style="margin-bottom: 15px; padding: 10px; background: #f5f5f5; border-radius: 4px;">
            <strong>Basic Configuration:</strong><br/>
            OnlyOffice URL: ${this.onlyOfficeUrl}<br/>
            Internal URL: ${internalUrlInfo.message}<br/>
            Base URL: ${baseURL}<br/>
            Source: ${this.source}<br/>
            Path: ${this.path}<br/>
            Filename: ${filename}<br/>
            ${isShare ? `Share Hash: ${state.share.hash}` : 'User Request'}<br/>
          </div>

          ${configDetailsHtml}

          <div style="margin-bottom: 15px;">
            <strong>Process Steps:</strong><br/>
            ${this.debugInfo}
          </div>

          ${overallStatus}
        </div>
      `;

      state.tooltip.content = content;
      state.tooltip.show = true;
      state.tooltip.pointerEvents = true; // Enable clicks for troubleshooting links
      state.tooltip.width = "50vw"; // Wider for debug info
      state.tooltip.x = 50;
      state.tooltip.y = 50; // Position at top since content is longer now
    },

    showConfigDetails(configData) {
      if (!this.debugMode) return;

      // Store config for tooltip display
      this.clientConfig = configData;

      // Extract key URLs and domains
      const docUrl = configData.document ? configData.document.url : 'Not found';
      const callbackUrl = configData.editorConfig ? configData.editorConfig.callbackUrl : 'Not found';

      let urlAnalysis = "";
      try {
        if (docUrl !== 'Not found') {
          const downloadDomain = new URL(docUrl).origin;
          urlAnalysis += `Download Domain: ${downloadDomain}<br/>`;
        }
        if (callbackUrl !== 'Not found') {
          const callbackDomain = new URL(callbackUrl).origin;
          urlAnalysis += `Callback Domain: ${callbackDomain}<br/>`;
        }
      } catch (error) {
        urlAnalysis = "⚠️ URL parsing error - check configuration<br/>";
      }

      this.addDebugStep("Config Analysis", "success", urlAnalysis + "URLs generated for OnlyOffice server");
    },

    checkOnlyOfficeServer() {
      if (!this.debugMode) return;

      const testUrl = `${this.onlyOfficeUrl}/web-apps/apps/api/documents/api.js`;

      this.updateDebugStatus("🔄 Testing OnlyOffice server connectivity...");

      // Monitor browser console for OnlyOffice errors
      const originalConsoleError = console.error;
      let detectedNetworkError = false;

      console.error = (...args) => {
        originalConsoleError.apply(console, args);

        const errorStr = args.join(' ');
        if (errorStr.includes('ERR_CONNECTION_REFUSED') && errorStr.includes(this.onlyOfficeUrl)) {
          detectedNetworkError = true;
          this.hasErrors = true;

          let errorMsg = `❌ OnlyOffice Server Connection Refused<br/>`;
          errorMsg += `<strong>Failed URL:</strong> ${errorStr.match(/GET ([^\s]+)/)?.[1] || testUrl}<br/>`;
          errorMsg += `<strong>OnlyOffice Server:</strong> ${this.onlyOfficeUrl}<br/><br/>`;
          errorMsg += `<strong>This means:</strong><br/>`;
          errorMsg += `• OnlyOffice server is <strong>NOT RUNNING</strong><br/>`;
          errorMsg += `• Wrong OnlyOffice URL configured<br/>`;
          errorMsg += `• Network/firewall blocking connection<br/><br/>`;
          errorMsg += `<strong>To fix:</strong><br/>`;
          errorMsg += `• Start OnlyOffice Document Server<br/>`;
          errorMsg += `• Verify ${this.onlyOfficeUrl} is accessible<br/>`;
          errorMsg += `• Check Docker containers if using Docker<br/>`;
          errorMsg += `<br/><a href="${wikiLink}" target="_blank" style="color: #1976d2;">${wikiLinkText}</a>`;

          this.updateDebugStatus(errorMsg);
        }
      };

      // Test basic connectivity
      setTimeout(() => {
        fetch(testUrl, { method: 'HEAD', mode: 'no-cors' })
        .then(() => {
          if (!detectedNetworkError) {
            this.updateDebugStatus("✅ OnlyOffice server appears reachable");
          }
        })
        .catch(() => {
          if (!detectedNetworkError) {
            this.hasErrors = true;
            this.updateDebugStatus(`❌ OnlyOffice server test failed: ${testUrl}<br/>• Check if OnlyOffice server is running<br/>• Verify URL configuration`);
          }
        });

        // Monitor for successful completion
        setTimeout(() => {
          console.error = originalConsoleError; // Restore original console.error

          if (!this.hasErrors && !detectedNetworkError) {
            this.updateDebugStatus("✅ Document Download - Complete");
            this.updateDebugStatus("🎉 Editor Initialization - Success! All steps completed.");
          }
        }, 4000);

      }, 1000);
    },

    onLoadComponentError(errorInfo) {
      console.error("OnlyOffice component load error:", errorInfo);
      if (this.debugMode) {
        this.hasErrors = true;

        let errorMsg = "❌ OnlyOffice Component Load Error<br/>";

        if (errorInfo === -2) {
          errorMsg += "<strong>Error Code -2:</strong> OnlyOffice server connection failed<br/>";
          errorMsg += "• Verify OnlyOffice server is running<br/>";
          errorMsg += `• Test URL manually: ${this.onlyOfficeUrl}<br/>`;
          errorMsg += "• Check network connectivity<br/>";
        } else if (errorInfo === -3) {
          errorMsg += "<strong>Error Code -3:</strong> Document loading failed<br/>";
          errorMsg += "• File may be corrupted or unsupported<br/>";
          errorMsg += "• Check file permissions<br/>";
        } else if (errorInfo === -4) {
          errorMsg += "<strong>Error Code -4:</strong> Document download failed<br/>";
          errorMsg += "• Authentication may have expired<br/>";
          errorMsg += "• Check download URL accessibility<br/>";
          if (!this.getInternalUrlInfo().isSet) {
            errorMsg += "• <strong>Suggestion:</strong> consider adding `server.internalUrl` if your OnlyOffice service is on the same network as FileBrowser<br/>";
          }
        } else {
          errorMsg += `<strong>Error Code:</strong> ${errorInfo || 'Unknown error'}<br/>`;
          errorMsg += "• Check browser console for detailed error messages<br/>";
        }

        errorMsg += `<br/><a href="${wikiLink}" target="_blank" style="color: #1976d2;">${wikiLinkText}</a>`;

        this.updateDebugStatus(errorMsg);
      }
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
