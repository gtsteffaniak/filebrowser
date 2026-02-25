<template>
  <div class="card-content no-buttons">
    <div class="debug-tooltip">
      <div class="debug-header">
        <h3 class="debug-title">🔧 {{ $t("onlyoffice.debugTitle") }}</h3>
      </div>
      <div class="debug-section debug-section-basic">
        <div class="debug-section-header">
          <strong class="debug-section-title">⚙️ {{ $t("onlyoffice.basicConfiguration") }}</strong>
        </div>
        {{ $t("onlyoffice.onlyOffice") }} {{ $t("general.path", { suffix: ":" }) }} {{ onlyOfficeUrl }}<br/>
        {{ $t("onlyoffice.internalUrl") }}: {{ internalUrlInfo.message }}<br/>
        {{ $t("general.path", { suffix: ":" }) }} {{ baseURL }}<br/>
        {{ $t("general.source", { suffix: ":" }) }} {{ source }}<br/>
        {{ $t("general.path", { suffix: ":" }) }} {{ path }}<br/>
        {{ $t("general.name", { suffix: ":" }) }} {{ filename }}<br/>
        {{ isShare ? `${$t("onlyoffice.shareHash")}: ${shareHash}` : $t("onlyoffice.userRequest") }}<br/>
      </div>

      <div v-if="clientConfig && clientConfig.document" class="debug-section debug-section-config">
        <div class="debug-section-header">
          <strong class="debug-section-title">🔧 {{ $t("onlyoffice.configurationDetails") }}</strong>
        </div>
        {{ $t("onlyoffice.documentKey") }}: {{ clientConfig.document.key }}<br/>
        {{ $t("onlyoffice.fileType") }}: {{ clientConfig.document.fileType }}<br/>
        {{ $t("onlyoffice.editMode") }}: {{ clientConfig.editorConfig ? clientConfig.editorConfig.mode : 'N/A' }}<br/>
        {{ $t("onlyoffice.downloadURL") }}: {{ clientConfig.document.url ? clientConfig.document.url.substring(0, 80) + '...' : 'N/A' }}<br/>
        {{ $t("onlyoffice.callbackURL") }}: {{ clientConfig.editorConfig && clientConfig.editorConfig.callbackUrl ? clientConfig.editorConfig.callbackUrl.substring(0, 80) + '...' : 'N/A' }}<br/>
        <br/>
        <strong>{{ $t("onlyoffice.networkFlow") }}:</strong><br/>
        {{ $t("onlyoffice.browser") }} ({{ windowOrigin }}) ↔ {{ $t("onlyoffice.onlyOffice") }}: {{ onlyOfficeUrl }}<br/>
        {{ $t("onlyoffice.onlyOffice") }} → {{ $t("onlyoffice.fileBrowser") }}: {{ downloadDomain }}<br/>
      </div>

      <div class="debug-section">
        <strong>{{ $t("onlyoffice.processSteps") }}:</strong><br/>
        <div v-html="debugInfo"></div>
      </div>

      <div v-if="onlyOfficeLogs.length > 0" class="debug-section debug-section-logs">
        <div class="debug-section-header-logs">
          <strong class="debug-section-title-logs">📋 {{ $t("onlyoffice.backendLogs") }}</strong>
          <span class="debug-section-counter">{{ onlyOfficeLogs.length }} {{ $t("onlyoffice.entries") }}</span>
        </div>
        <div v-for="log in displayedLogs" :key="log.id" class="debug-log-entry">
          <span class="debug-log-level" :style="{ color: getLogLevelColor(log.level) }">[{{ log.level }}]</span>
          <span class="debug-log-timestamp">{{ formatTimestamp(log.timestamp) }}</span>
          <span class="debug-log-component">[{{ log.component }}]</span>
          <span class="debug-log-message">{{ log.message }}</span>
        </div>
      </div>
      <div v-else class="debug-section">
        {{ noLogsMessage }}
      </div>

      <div v-if="overallStatus" v-html="overallStatus"></div>
    </div>
  </div>
</template>

<script>
import { mutations, state, getters } from "@/store";
import { globalVars } from "@/utils/constants";
import { officeApi } from "@/api";
import { events } from "@/notify";

const wikiLink = "https://github.com/gtsteffaniak/filebrowser/wiki/Office-Support#onlyoffice-integration-troubleshooting-guide";
const wikiLinkText = "📖 View Troubleshooting Guide";

export default {
  name: "officeDebug",
  props: {
    onlyOfficeUrl: {
      type: String,
      required: true,
    },
    source: {
      type: String,
      required: true,
    },
    path: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      debugInfo: `⏳ ${this.$t("onlyoffice.startingInit")}`,
      clientConfig: {},
      hasErrors: false,
      onlyOfficeLogs: [],
      documentId: null,
      sseConnection: null,
    };
  },
  computed: {
    baseURL() {
      return globalVars.baseURL;
    },
    filename() {
      return this.path.split('/').pop() || "";
    },
    isShare() {
      return getters.isShare();
    },
    shareHash() {
      return state.shareInfo?.hash || "";
    },
    windowOrigin() {
      return window.location.origin;
    },
    internalUrlInfo() {
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
    downloadDomain() {
      if (this.clientConfig && this.clientConfig.document && this.clientConfig.document.url) {
        try {
          return new URL(this.clientConfig.document.url).origin;
        } catch (e) {
          return 'N/A';
        }
      }
      return 'N/A';
    },
    displayedLogs() {
      return this.onlyOfficeLogs.slice(-10);
    },
    noLogsMessage() {
      return !state.user.permissions.admin 
        ? this.$t("onlyoffice.logsNotAvailable")
        : this.$t("onlyoffice.waitingForLogs");
    },
    overallStatus() {
      if (this.hasErrors) {
        return `
          <div class="debug-status-error">
            <strong>❌ ${this.$t("onlyoffice.errorDetected")}</strong><br/>
            ${this.$t("onlyoffice.integrationFailed")}<br/>
            <a href="${wikiLink}" target="_blank">
              📖 ${this.$t("onlyoffice.troubleshootingGuide")}
            </a>
          </div>
        `;
      } else if (this.debugInfo.includes("🎉") && !this.hasErrors) {
        return `
          <div class="debug-status-success">
            <strong>🎉 ${this.$t("onlyoffice.documentInitialized")}</strong><br/>
            ${this.$t("onlyoffice.integrationSuccess")}
          </div>
        `;
      }
      return "";
    },
  },
  async mounted() {
    this.setupOnlyOfficeLogStreaming();
    this.monitorCallbackIssues();
    await this.initializeDebugger();
  },
  beforeUnmount() {
    if (this.sseConnection) {
      this.sseConnection.close();
      this.sseConnection = null;
    }
    window.removeEventListener('onlyOfficeLogEvent', this.handleOnlyOfficeLogEvent);
    
    if (state.user.permissions.admin && !state.user.permissions.realtime) {
      console.log('🔌 Cleaning up OnlyOffice SSE connection');
      events.stopSSE();
    }
  },
  methods: {
    async initializeDebugger() {
      try {
        this.updateDebugStatus(`✅ ${this.$t("onlyoffice.configCheck")}`);
        
        const configData = await officeApi.getConfig(state.req);
        this.clientConfig = configData;
        
        if (this.clientConfig.document && this.clientConfig.document.key) {
          this.documentId = this.clientConfig.document.key;
        }
        
        this.updateDebugStatus(`✅ ${this.$t("onlyoffice.apiRequest")}`);
        this.updateDebugStatus(`✅ ${this.$t("onlyoffice.serverConnection")}`);
        
        setTimeout(() => {
          this.checkOnlyOfficeServer();
        }, 100);
        
      } catch (error) {
        console.error("Error during OnlyOffice setup:", error);
        const errorMsg = (error && typeof error === 'object' && 'message' in error) ? error.message : String(error);
        this.updateDebugStatus(`❌ ${this.$t("onlyoffice.setupError")}: ${errorMsg}<br/><a href="${wikiLink}" target="_blank">📖 ${this.$t("onlyoffice.troubleshootingGuide")}</a>`);
      }
    },
    
    updateDebugStatus(message) {
      this.debugInfo = this.debugInfo + "<br/>" + message;
    },
    
    checkOnlyOfficeServer() {
      const testUrl = `${this.onlyOfficeUrl}/web-apps/apps/api/documents/api.js`;
      this.updateDebugStatus(`🔄 ${this.$t("onlyoffice.testingConnectivity")}`);

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
          errorMsg += `<br/><a href="${wikiLink}" target="_blank" style="color: #1976d2;">📖 ${this.$t("onlyoffice.troubleshootingGuide")}</a>`;

          this.updateDebugStatus(errorMsg);
        }
      };

      setTimeout(() => {
        fetch(testUrl, { method: 'HEAD', mode: 'no-cors' })
        .then(() => {
          if (!detectedNetworkError) {
            this.updateDebugStatus(`✅ ${this.$t("onlyoffice.serverReachable")}`);
          }
        })
        .catch(() => {
          if (!detectedNetworkError) {
            this.hasErrors = true;
            this.updateDebugStatus(`❌ OnlyOffice server test failed: ${testUrl}<br/>• Check if OnlyOffice server is running<br/>• Verify URL configuration`);
          }
        });

        setTimeout(() => {
          console.error = originalConsoleError;

          if (!this.hasErrors && !detectedNetworkError) {
            this.updateDebugStatus(`✅ ${this.$t("onlyoffice.documentDownload")}`);
            this.updateDebugStatus(`🎉 ${this.$t("onlyoffice.editorSuccess")}`);
          }
        }, 4000);
      }, 1000);
    },
    
    monitorCallbackIssues() {
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

      setTimeout(() => {
        window.fetch = originalFetch;
      }, 15000);
    },
    
    setupOnlyOfficeLogStreaming() {
      if (!state.user.permissions.admin) {
        return;
      }

      if (state.user.permissions.admin && !state.user.permissions.realtime) {
        console.log('🔗 Starting SSE connection for OnlyOffice admin user');
        events.startOnlyOfficeSSE();
      }

      this.handleOnlyOfficeLogEvent = this.handleOnlyOfficeLogEvent.bind(this);
      window.addEventListener('onlyOfficeLogEvent', this.handleOnlyOfficeLogEvent);
    },

    handleOnlyOfficeLogEvent(event) {
      const logData = event.detail;

      if (!this.documentId || logData.documentId === this.documentId) {
        this.addOnlyOfficeLog(logData);
      }
    },

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

      if (this.onlyOfficeLogs.length > 50) {
        this.onlyOfficeLogs = this.onlyOfficeLogs.slice(-50);
      }
    },
    
    getLogLevelColor(level) {
      switch (level) {
        case 'ERROR': return '#f44336';
        case 'WARN': return '#ff9800';
        case 'INFO': return '#4caf50';
        case 'DEBUG': return '#2196f3';
        default: return '#666';
      }
    },
    
    formatTimestamp(timestamp) {
      return new Date(timestamp).toLocaleTimeString();
    },
  },
};
</script>

<style scoped>
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

.debug-status-error :deep(a) {
  color: #1976d2;
}

.card-content {
  max-height: 70vh;
  overflow-y: auto;
}
</style>
