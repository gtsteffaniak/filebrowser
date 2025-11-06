<template>
  <!-- Conditionally render the DocumentEditor component -->
  <DocumentEditor v-if="ready" id="docEditor" :documentServerUrl="globalVars.onlyOfficeUrl" :config="clientConfig"
    :onLoadComponentError="onLoadComponentError" />
  <div v-else>
    <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
  </div>
  <div @click="close" class="floating-close button" :class="{ 'float-in': floatIn }">
    <i class="material-icons">close</i>
  </div>
</template>

<script>
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { globalVars } from "@/utils/constants";
import { state, getters, mutations } from "@/store";
import { removeLastDir } from "@/utils/url";
import { filesApi } from "@/api";
import { toStandardLocale } from "@/i18n";
import { events } from "@/notify";

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
      // OnlyOffice log streaming
      onlyOfficeLogs: [],
      documentId: null,
      sseConnection: null,
    };
  },
  computed: {
    req() {
      return state.req;
    },
    onlyOfficeUrl() {
      return globalVars.onlyOfficeUrl;
    },
    globalVars() {
      return globalVars;
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
      this.monitorCallbackIssues();
    }

    // Setup OnlyOffice log streaming early to catch events
    this.setupOnlyOfficeLogStreaming();

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

      // Extract document ID for log streaming
      if (this.clientConfig.document && this.clientConfig.document.key) {
        this.documentId = this.clientConfig.document.key;
        // Note: setupOnlyOfficeLogStreaming() is already called in mounted() for early setup
      }

      // if language is not en , set it to the current language
      if (state.user.locale !== "en") {
        this.clientConfig.editorConfig.lang = toStandardLocale(state.user.locale);
      }

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
  beforeUnmount() {
    this.destroyOnlyOffice();
  },
  methods: {
    // Clean up any existing OnlyOffice instances
    destroyOnlyOffice() {
      console.log('Cleaning up OnlyOffice...');
      // Remove all iframes
      const iframes = document.querySelectorAll('iframe');
      iframes.forEach(iframe => {
        if (iframe.src && iframe.src.includes('onlyoffice')) {
          iframe.remove();
        }
      });
      // Clean up global objects
      if (window.DocsAPI) delete window.DocsAPI;
      // Clean up SSE connection
      if (this.sseConnection) {
        this.sseConnection.close();
        this.sseConnection = null;
      }
      // Clean up global SSE event listener
      window.removeEventListener('onlyOfficeLogEvent', this.handleOnlyOfficeLogEvent);

      // Clean up SSE connection if it was started for OnlyOffice admin users
      if (state.user.permissions.admin && !state.user.permissions.realtime) {
        console.log('🔌 Cleaning up OnlyOffice SSE connection');
        events.stopSSE();
      }
    },
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

          <div style="margin-bottom: 15px; padding: 10px; border-radius: 4px;">
            <strong>Configuration:</strong><br/>
            OnlyOffice URL: ${this.onlyOfficeUrl}<br/>
            Internal URL: ${this.getInternalUrlInfo().message}<br/>
            Base URL: ${globalVars.baseURL}<br/>
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
          <div class="debug-section debug-section-config">
            <div class="debug-section-header">
              <strong class="debug-section-title">🔧 OnlyOffice Configuration Details</strong>
            </div>
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
          <div class="debug-status-error">
            <strong>❌ Error Detected</strong><br/>
            OnlyOffice integration failed. Check the failed steps above.<br/>
            <a href="${wikiLink}" target="_blank">
              ${wikiLinkText}
            </a>
          </div>
        `;
      } else if (this.debugInfo.includes("🎉") && !this.hasErrors) {
        overallStatus = `
          <div class="debug-status-success">
            <strong>🎉 Document Initialized</strong><br/>
            OnlyOffice integration intialized successfully. Monitor backend logs for any issues.
          </div>
        `;
      }

      const noLogsMessage = !state.user.permissions.admin ? "Backend logs are not available -- user must be admin to view backend logs" : "Waiting for backend logs...";

      let content = `
        <div class="debug-tooltip">
          <div class="debug-header">
            <h3 class="debug-title">🔧 OnlyOffice Debug Trace</h3>
            <button class="button tooltip-close-button" onclick="window.closeTooltip()" >x</button>
          </div>

          <div class="debug-section debug-section-basic">
            <div class="debug-section-header">
              <strong class="debug-section-title">⚙️ Basic Configuration</strong>
            </div>
            OnlyOffice URL: ${this.onlyOfficeUrl}<br/>
            Internal URL: ${internalUrlInfo.message}<br/>
            Base URL: ${globalVars.baseURL}<br/>
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

          ${this.onlyOfficeLogs.length > 0 ? `
            <div class="debug-section debug-section-logs">
              <div class="debug-section-header-logs">
                <strong class="debug-section-title-logs">📋 Backend Logs</strong>
                <span class="debug-section-counter">${this.onlyOfficeLogs.length} entries</span>
              </div>
              ${this.onlyOfficeLogs.slice(-10).map(log =>
                `<div class="debug-log-entry">
                  <span class="debug-log-level" style="color: ${this.getLogLevelColor(log.level)};">[${log.level}]</span>
                  <span class="debug-log-timestamp">${new Date(log.timestamp).toLocaleTimeString()}</span>
                  <span class="debug-log-component">[${log.component}]</span>
                  <span class="debug-log-message">${log.message}</span>
                </div>`
              ).join('')}
            </div>
          ` : noLogsMessage}

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

        // Monitor for successful completion and server-to-server communication
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
        let suggestions = [];

        if (errorInfo === -1) {
          errorMsg += "<strong>Error Code -1:</strong> Unknown error loading OnlyOffice component<br/>";
          suggestions = [
            "Check browser console for detailed error messages",
            "Verify OnlyOffice server is accessible",
            "Check if OnlyOffice server is properly configured",
            "Try refreshing the page"
          ];
        } else if (errorInfo === -2) {
          errorMsg += "<strong>Error Code -2:</strong> Failed to load DocsAPI from OnlyOffice server<br/>";
          suggestions = [
            "Verify OnlyOffice server is running and accessible",
            `Test OnlyOffice server URL manually: ${this.onlyOfficeUrl}`,
            "Check network connectivity between browser and OnlyOffice server",
            "Verify OnlyOffice server is not behind authentication",
            "Check if OnlyOffice server is fully initialized (wait a few minutes after startup)"
          ];
        } else if (errorInfo === -3) {
          errorMsg += "<strong>Error Code -3:</strong> DocsAPI is not defined (OnlyOffice server not responding)<br/>";
          suggestions = [
            "OnlyOffice server is not responding or not properly configured",
            "Check OnlyOffice server logs for startup errors",
            "Verify OnlyOffice server URL is correct and accessible",
            "Ensure OnlyOffice server has completed initialization",
            "Check if OnlyOffice server is behind a reverse proxy with incorrect configuration"
          ];
        } else if (errorInfo === -4) {
          errorMsg += "<strong>Error Code -4:</strong> Document download failed<br/>";
          suggestions = [
            "Authentication may have expired - try refreshing",
            "Check download URL accessibility",
            "Verify OnlyOffice server can reach FileBrowser",
            "Check network connectivity between OnlyOffice and FileBrowser"
          ];

          if (!this.getInternalUrlInfo().isSet) {
            suggestions.push("Consider adding `server.internalUrl` if OnlyOffice is on same network");
          }
        } else if (errorInfo === -5) {
          errorMsg += "<strong>Error Code -5:</strong> Document security error<br/>";
          suggestions = [
            "Check if document is password protected",
            "Verify OnlyOffice server security settings",
            "Check OnlyOffice server logs for security errors",
            "Verify document format is supported"
          ];
        } else if (errorInfo === -6) {
          errorMsg += "<strong>Error Code -6:</strong> Document access denied<br/>";
          suggestions = [
            "Check user permissions in FileBrowser",
            "Verify OnlyOffice server has proper authentication",
            "Check if file path contains special characters",
            "Verify OnlyOffice server configuration"
          ];
        } else {
          errorMsg += `<strong>Error Code:</strong> ${errorInfo || 'Unknown error'}<br/>`;
          suggestions = [
            "Check browser console for detailed error messages",
            "Try refreshing the page",
            "Check OnlyOffice server logs"
          ];
        }

        // Add suggestions as bullet points
        errorMsg += "<br/><strong>Suggestions:</strong><br/>";
        suggestions.forEach(suggestion => {
          errorMsg += `• ${suggestion}<br/>`;
        });

        // Add configuration check
        const configIssues = this.checkConfigurationIssues();
        if (configIssues.length > 0) {
          errorMsg += "<br/><strong>Configuration Issues Detected:</strong><br/>";
          configIssues.forEach(issue => {
            errorMsg += `• ${issue}<br/>`;
          });
        }

        errorMsg += `<br/><a href="${wikiLink}" target="_blank" style="color: #1976d2;">${wikiLinkText}</a>`;

        this.updateDebugStatus(errorMsg);
      }
    },

    checkConfigurationIssues() {
      const issues = [];

      // Check OnlyOffice URL
      if (!this.onlyOfficeUrl || this.onlyOfficeUrl === '') {
        issues.push("OnlyOffice URL is not configured");
      } else if (!this.onlyOfficeUrl.startsWith('http')) {
        issues.push("OnlyOffice URL should start with http:// or https://");
      }

      // Check internal URL configuration
      const internalUrlInfo = this.getInternalUrlInfo();
      if (internalUrlInfo.isSet && internalUrlInfo.message.includes('Error')) {
        issues.push("Internal URL configuration has parsing errors");
      }

      // Check document URL accessibility
      if (this.clientConfig && this.clientConfig.document && this.clientConfig.document.url) {
        try {
          const docUrl = new URL(this.clientConfig.document.url);
          if (docUrl.protocol !== window.location.protocol) {
            issues.push("Document URL protocol mismatch with current page");
          }
        } catch (e) {
          issues.push("Document URL is malformed");
        }
      }

      // Check callback URL
      if (this.clientConfig && this.clientConfig.editorConfig && this.clientConfig.editorConfig.callbackUrl) {
        try {
          const callbackUrl = new URL(this.clientConfig.editorConfig.callbackUrl);
          if (callbackUrl.protocol !== window.location.protocol) {
            issues.push("Callback URL protocol mismatch with current page");
          }
        } catch (e) {
          issues.push("Callback URL is malformed");
        }
      }

      return issues;
    },

    // Monitor for callback-related issues and server-to-server communication problems
    monitorCallbackIssues() {
      if (!this.debugMode) return;

      // Monitor for callback errors in the network tab
      const originalFetch = window.fetch;
      window.fetch = (...args) => {
        return originalFetch.apply(window, args).then(response => {
          if (response.url.includes('onlyoffice/callback')) {
            if (response.status === 405) {
              this.hasErrors = true;
              this.updateDebugStatus(`
                ❌ Callback Method Not Allowed (405)<br/>
                <strong>Issue:</strong> OnlyOffice server is sending ${args[1]?.method || 'GET'} requests to callback URL<br/>
                <strong>Expected:</strong> POST requests<br/>
                <strong>Root Cause:</strong> OnlyOffice server configuration issue<br/>
                <strong>Solution:</strong><br/>
                • Check OnlyOffice server callback configuration<br/>
                • Verify callback URL is correctly set in OnlyOffice server<br/>
                • Ensure OnlyOffice server is using correct HTTP method<br/>
                • Check OnlyOffice server logs for callback configuration errors<br/>
              `);
            } else if (response.status === 400) {
              this.hasErrors = true;
              this.updateDebugStatus(`
                ❌ Callback Bad Request (400)<br/>
                <strong>Issue:</strong> OnlyOffice server sent invalid callback data<br/>
                <strong>Root Cause:</strong> OnlyOffice server → FileBrowser API communication problem<br/>
                <strong>Solution:</strong><br/>
                • Check OnlyOffice server logs for callback generation errors<br/>
                • Verify JWT secret matches between OnlyOffice and FileBrowser<br/>
                • Check OnlyOffice server callback URL configuration<br/>
                • Ensure OnlyOffice server can reach FileBrowser internal URL<br/>
              `);
            } else if (response.status === 500) {
              this.hasErrors = true;
              this.updateDebugStatus(`
                ❌ Callback Internal Server Error (500)<br/>
                <strong>Issue:</strong> FileBrowser API error processing OnlyOffice callback<br/>
                <strong>Root Cause:</strong> FileBrowser API configuration or processing issue<br/>
                <strong>Solution:</strong><br/>
                • Check FileBrowser server logs for callback processing errors<br/>
                • Verify FileBrowser API configuration<br/>
                • Check if OnlyOffice server is sending valid callback data<br/>
                • Ensure FileBrowser has proper permissions to save files<br/>
              `);
            }
          }
          return response;
        });
      };

      // Monitor for JWT parsing errors and other callback issues
      const originalConsoleError = console.error;
      console.error = (...args) => {
        originalConsoleError.apply(console, args);

        const errorStr = args.join(' ');
        if (errorStr.includes('JWT') && errorStr.includes('callback')) {
          this.hasErrors = true;
          this.updateDebugStatus(`
            ❌ JWT Parsing Error in Callback<br/>
            <strong>Issue:</strong> OnlyOffice callback JWT token cannot be parsed<br/>
            <strong>Root Cause:</strong> OnlyOffice server → FileBrowser API JWT communication problem<br/>
            <strong>Solution:</strong><br/>
            • Check OnlyOffice server JWT configuration<br/>
            • Verify JWT secret matches between OnlyOffice and FileBrowser<br/>
            • Check OnlyOffice server logs for JWT generation errors<br/>
            • Ensure OnlyOffice server is using correct JWT signing method<br/>
          `);
        } else if (errorStr.includes('callback') && (errorStr.includes('network') || errorStr.includes('fetch'))) {
          this.hasErrors = true;
          this.updateDebugStatus(`
            ❌ Callback Network Error<br/>
            <strong>Issue:</strong> OnlyOffice server cannot reach FileBrowser callback URL<br/>
            <strong>Root Cause:</strong> Network connectivity between OnlyOffice server and FileBrowser API<br/>
            <strong>Solution:</strong><br/>
            • Check if OnlyOffice server can reach FileBrowser internal URL<br/>
            • Verify callback URL is accessible from OnlyOffice server<br/>
            • Check network configuration between services<br/>
            • Ensure FileBrowser internal URL is correctly configured<br/>
          `);
        }
      };

      // Monitor for document save failures (indicates server-to-server communication issues)
      this.monitorDocumentSaveFailures();

      // Restore original functions after 15 seconds
      setTimeout(() => {
        window.fetch = originalFetch;
        console.error = originalConsoleError;
      }, 15000);
    },

    // Monitor for document save failures which indicate server-to-server communication issues
    monitorDocumentSaveFailures() {
      if (!this.debugMode) return;

      // Track if document was opened but never saved
      let documentOpened = false;
      let saveAttempts = 0;
      let lastSaveAttempt = 0;

      // Monitor for document ready event
      const checkDocumentReady = () => {
        if (this.clientConfig && this.clientConfig.document) {
          documentOpened = true;
          console.log("OnlyOffice debug: Document opened successfully");
        }
      };

      // Monitor for save attempts
      const checkSaveAttempts = () => {
        if (documentOpened) {
          saveAttempts++;
          lastSaveAttempt = Date.now();
          console.log(`OnlyOffice debug: Save attempt #${saveAttempts}`);
        }
      };

      // Check for save failures after 30 seconds
      setTimeout(() => {
        if (documentOpened && saveAttempts === 0) {
          this.hasErrors = true;
          this.updateDebugStatus(`
            ❌ Document Save Issues Detected<br/>
            <strong>Issue:</strong> Document opened but no save attempts detected<br/>
            <strong>Root Cause:</strong> OnlyOffice server → FileBrowser API communication problem<br/>
            <strong>Possible Causes:</strong><br/>
            • OnlyOffice server cannot reach FileBrowser callback URL<br/>
            • Callback URL is incorrect or inaccessible<br/>
            • JWT secret mismatch between OnlyOffice and FileBrowser<br/>
            • Network connectivity issues between services<br/>
            <strong>Solution:</strong><br/>
            • Check OnlyOffice server logs for callback errors<br/>
            • Verify callback URL is accessible from OnlyOffice server<br/>
            • Test callback URL manually from OnlyOffice server<br/>
            • Check FileBrowser internal URL configuration<br/>
          `);
        }
      }, 30000);

      // Expose monitoring functions globally for debugging
      window.onlyOfficeDebug = {
        checkDocumentReady,
        checkSaveAttempts,
        getStats: () => ({ documentOpened, saveAttempts, lastSaveAttempt })
      };
    },
    // Setup SSE connection for OnlyOffice logs
    setupOnlyOfficeLogStreaming() {
      // Allow log streaming for admin users or when in debug mode
      if (!state.user.permissions.admin && !this.debugMode) {
        return;
      }

      // Start SSE connection for admin users even if they don't have realtime permissions
      if (state.user.permissions.admin && !state.user.permissions.realtime) {
        console.log('🔗 Starting SSE connection for OnlyOffice admin user');
        events.startOnlyOfficeSSE();
      }

      // Setup the global SSE listener (documentId will be set later if not available yet)
      this.setupGlobalSSEListener();
    },

    // Setup listener for global SSE events
    setupGlobalSSEListener() {
      // Bind the event handler to this component instance
      this.handleOnlyOfficeLogEvent = this.handleOnlyOfficeLogEvent.bind(this);

      // Listen for custom events that we'll dispatch from the global SSE system
      window.addEventListener('onlyOfficeLogEvent', this.handleOnlyOfficeLogEvent);
    },

    // Handle OnlyOffice log events from global SSE system
    handleOnlyOfficeLogEvent(event) {
      const logData = event.detail;

      // If documentId is not set yet, store the log for later (this can happen during early setup)
      if (!this.documentId) {
        this.addOnlyOfficeLog(logData);
        return;
      }

      // Filter logs for this document
      if (logData.documentId === this.documentId) {
        this.addOnlyOfficeLog(logData);
      }
    },

    // Add OnlyOffice log to the display
    addOnlyOfficeLog(logData) {

      const logEntry = {
        id: Date.now() + Math.random(),
        timestamp: logData.timestamp,
        level: logData.logLevel,
        component: logData.component,
        message: logData.message,
        username: logData.username,
        sessionId: logData.sessionId
      };

      this.onlyOfficeLogs.push(logEntry);
      console.log("OnlyOffice debug: Total logs now:", this.onlyOfficeLogs.length);

      // Keep only last 50 logs to prevent memory issues
      if (this.onlyOfficeLogs.length > 50) {
        this.onlyOfficeLogs = this.onlyOfficeLogs.slice(-50);
      }

      // Note: Log display is handled by the dedicated Backend Logs section
      // No need to add to Process Steps section

      // Force update the debug tooltip to show the new logs
      this.updateDebugTooltip();
    },

    // Get color for log level
    getLogLevelColor(level) {
      switch (level) {
        case 'ERROR': return '#f44336';
        case 'WARN': return '#ff9800';
        case 'INFO': return '#4caf50';
        case 'DEBUG': return '#2196f3';
        default: return '#666';
      }
    },
  },
};
</script>

<style >
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

/* Debug tooltip styles */
.debug-tooltip {
  font-family: monospace;
  font-size: 11px;
  line-height: 1.4;
}

.debug-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.debug-title {
  margin: 0;
  color: #2196F3;
}

.debug-section {
  margin-bottom: 15px;
  padding: 10px;
  border-radius: 4px;
}

.debug-section-basic {
  background: #424242;
  color: white;
}

.debug-section-config {
  background: #424242;
  color: white;
}

.debug-section-logs {
  background: #f5f5f5;
  max-height: 200px;
  overflow-y: auto;
}

.debug-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding-bottom: 5px;
  border-bottom: 1px solid rgba(255,255,255,0.3);
}

.debug-section-header-logs {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding-bottom: 5px;
  border-bottom: 1px solid #ddd;
}

.debug-section-title {
  color: white;
}

.debug-section-title-logs {
  color: #2196F3;
}

.debug-section-counter {
  font-size: 10px;
  color: #666;
}

.debug-log-entry {
  margin: 2px 0;
  font-size: 10px;
  font-family: monospace;
  padding: 2px 4px;
  border-radius: 2px;
  background: rgba(255,255,255,0.5);
  color: #000;
}

.debug-log-level {
  font-weight: bold;
}

.debug-log-timestamp {
  color: #666;
  margin-left: 4px;
}

.debug-log-component {
  color: #2196F3;
  margin-left: 4px;
}

.debug-log-message {
  color: #000;
  margin-left: 4px;
}

.debug-status-success {
  margin-top: 15px;
  padding: 10px;
  background: #e8f5e8;
  border-radius: 4px;
  color: #2e7d32;
}

.debug-status-error {
  margin-top: 15px;
  padding: 10px;
  background: #ffebee;
  border-radius: 4px;
  color: #c62828;
}

.debug-status-error a {
  color: #1976d2;
}
</style>
