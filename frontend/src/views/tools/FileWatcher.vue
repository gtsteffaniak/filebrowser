<template>
  <div class="file-watcher">
    <div class="card file-watcher-config">
      <div class="card-content config-row" :class="{ 'mobile': isMobile }">
        <div class="config-item file-picker">
          <div aria-label="file-watcher-file" class="searchContext clickable button file-picker-button" @click="openPathPicker">
            {{ getFilePathText }}
          </div>
        </div>
        <div class="config-row-second" :class="{ 'mobile': isMobile }">
          <div class="config-item interval-select">
            <select v-model="selectedInterval" class="input" :disabled="watching">
              <option v-for="interval in availableIntervals" :key="interval.value" :value="interval.value" :disabled="interval.disabled">
                {{ interval.label }}
              </option>
            </select>
          </div>
          <div class="config-item lines-input">
            <label class="lines-label">{{ $t('general.lines',{suffix: ':'}) }}</label>
            <input class="sizeInput input" v-model.number="selectedLines" type="number" min="1" max="50"
              :placeholder="$t('general.number')" :disabled="watching" />
          </div>
          <div class="config-item play-button">
            <button @click="toggleWatch" class="button" :disabled="!watching && !canStart">
              <i v-if="watching" class="material-icons">pause</i>
              <i v-else class="material-icons">play_arrow</i>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-content file-watcher-output">
        <div v-if="error" class="error-message">
          {{ error }}
        </div>
        <div class="terminal-header boarder-radius" :class="{ 'mobile': isMobile }">
          <div class="header-row header-row-first">
            <div class="header-left"></div>
            <div v-if="fileName" class="header-center">
              <span class="header-filename">{{ fileName }}</span>
              <span v-if="fileSize" class="header-filesize">({{ fileSize }})</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            </div>
            <div class="header-right"></div>
          </div>
          <!-- Mobile: separate row for last modified -->
          <div v-if="isMobile && fileModified" class="header-row header-row-second-mobile">
            <div class="header-center-mobile">
              <span class="header-label">{{ $t('files.lastModified',{suffix: ':'}) }}</span>
              <span class="header-value">{{ fileModified }}</span>
            </div>
          </div>
          <!-- Desktop: combined row OR Mobile: third row -->
          <div class="header-row" :class="isMobile ? 'header-row-third-mobile' : 'header-row-second'">
            <div class="header-left">
              <span class="header-label">{{ $t('general.latency',{suffix: ':'}) }}</span>
              <span class="header-value" :class="latencyClass">
                {{ watching ? `${currentLatency}ms` : '-' }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </span>
            </div>
            <div v-if="!isMobile && fileModified" class="header-center">
              <span class="header-label">{{ $t('files.lastModified',{suffix: ':'}) }}</span>
              <span class="header-value">{{ fileModified }}</span>
            </div>
            <div v-if="lastUpdateTime" class="header-right">
              <span class="header-label">{{ $t('general.lastUpdate',{suffix: ':'}) }}</span>
              <span class="header-value">{{ getRelativeUpdateTime }}</span>
            </div>
          </div>
        </div>
        <div ref="terminalOutput" class="terminal-output boarder-radius" :class="{ 'dark-mode': isDarkMode }">
          <div v-for="(line, index) in outputLines" :key="index" class="terminal-line">
            <span class="terminal-text">{{ line.text }}</span>
          </div>
          <div v-if="outputLines.length === 0 && !watching" class="empty-state">
            <i class="material-icons">terminal</i>
            <p>{{ $t('tools.fileWatcher.emptyState') }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import { getApiPath } from "@/utils/url";
import { fetchURL } from "@/api/utils";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { formatTimestamp, fromNow } from "@/utils/moment";

export default {
  name: "FileWatcher",
  data() {
    return {
      selectedSource: "",
      filePath: "/",
      selectedInterval: 5,
      selectedLines: 10,
      watching: false,
      outputLines: [],
      error: null,
      pollInterval: null,
      useRealtime: false,
      eventSource: null,
      isInitializing: true,
      currentLatency: 0,
      fileSize: null,
      fileName: null,
      fileModified: null,
      lastUpdateTime: null,
      updateTimer: null,
      currentTime: Date.now(),
      latencyPingInterval: null,
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
    isMobile() {
      return state.isMobile;
    },
    canStart() {
      return this.selectedSource && this.filePath && !this.watching && !this.isLikelyDirectory;
    },
    availableIntervals() {
      const realtimePerm = state.user?.permissions?.realtime;
      const hasRealtime = realtimePerm === true;
      return [
        { value: 1, label: "1s", disabled: !hasRealtime },
        { value: 2, label: "2s", disabled: !hasRealtime },
        { value: 5, label: "5s", disabled: false },
        { value: 10, label: "10s", disabled: false },
        { value: 15, label: "15s", disabled: false },
        { value: 30, label: "30s", disabled: false },
      ];
    },
    getFilePathText() {
      if (this.filePath && this.filePath !== "/") {
        return `${this.$t('general.file', { suffix: ':' })} ${this.filePath}`;
      }
      return this.$t('tools.fileWatcher.chooseFile');
    },
    getRelativeUpdateTime() {
      if (!this.lastUpdateTime) return '';
      // Use currentTime to force re-computation every second
      this.currentTime; // This creates a dependency
      return fromNow(this.lastUpdateTime, state.user?.locale || 'en');
    },
    latencyClass() {
      // Check watching state first - if not watching, always return inactive
      if (this.watching === false || !this.watching) {
        return 'latency-inactive'; // Gray for inactive
      }
      // Only check latency thresholds when actively watching
      if (this.currentLatency < 300) return 'latency-good'; // Green
      if (this.currentLatency < 1000) return 'latency-ok'; // Yellow
      return 'latency-slow'; // Red
    },
    isLikelyDirectory() {
      return this.filePath && this.filePath.endsWith("/");
    },
  },
  watch: {
    filePath() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    selectedSource() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    selectedInterval(newVal) {
      // Validate interval - if user doesn't have realtime and selected 1 or 2, change to 5
      const validated = this.validateInterval(newVal);
      if (validated !== newVal) {
        this.selectedInterval = validated;
        return; // Don't proceed with update, let the watcher fire again with corrected value
      }
      
      // If watching and interval changed, restart watching
      if (this.watching) {
        this.stopWatch();
        this.$nextTick(() => {
          this.startWatch();
        });
      }
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    selectedLines() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    '$route.query'() {
      if (!this.isInitializing) {
        this.initializeFromQuery();
      }
    },
    // Watch for user permission changes and validate interval
    'state.user.permissions'() {
      if (!this.isInitializing) {
        this.selectedInterval = this.validateInterval(this.selectedInterval);
      }
    },
  },
  mounted() {
    // Initialize from URL query parameters
    this.initializeFromQuery();
    
    // Validate and correct interval based on permissions
    this.selectedInterval = this.validateInterval(this.selectedInterval);
    
    // Set default source if not provided
    if (!this.selectedSource) {
      if (state.sources.current) {
        this.selectedSource = state.sources.current;
      } else if (state.sources.info && Object.keys(state.sources.info).length > 0) {
        this.selectedSource = Object.keys(state.sources.info)[0];
      }
    }
    
    // Mark initialization as complete
    this.isInitializing = false;
    this.updateUrl();

    // Listen for path selection
    eventBus.on('pathSelected', this.handlePathSelected);

    // Start timer to update relative time every second
    this.updateTimer = setInterval(() => {
      this.currentTime = Date.now();
    }, 1000);

    document.addEventListener('keydown', this.handleKeydown);
  },
  beforeUnmount() {
    eventBus.off('pathSelected', this.handlePathSelected);
    this.stopWatch();
    // Clear update timer
    if (this.updateTimer) {
      clearInterval(this.updateTimer);
      this.updateTimer = null;
    }
    // Clear latency ping interval
    if (this.latencyPingInterval) {
      clearInterval(this.latencyPingInterval);
      this.latencyPingInterval = null;
    }
    // Clear event listener
    document.removeEventListener('keydown', this.handleKeydown);
  },
  methods: {
    validateInterval(interval) {
      const realtimePerm = state.user?.permissions?.realtime;
      const hasRealtime = realtimePerm === true;
      // If user doesn't have realtime permissions and interval is 1 or 2, change to 5
      if (!hasRealtime && (interval === 1 || interval === 2)) {
        return 5;
      }
      return interval;
    },
    openPathPicker() {
      mutations.showHover({
        name: "pathPicker",
        props: {
          currentPath: this.filePath || "/",
          currentSource: this.selectedSource || state.sources.current || "",
          showFiles: true, // Show both: Directories and Files
        }
      });
    },
    handlePathSelected(data) {
      if (data && data.path !== undefined) {
        this.filePath = data.path;
      }
      if (data && data.source !== undefined) {
        this.selectedSource = data.source;
      }
      mutations.closeHovers();
    },
    initializeFromQuery() {
      const query = this.$route.query;

      // Update filePath from URL - if path is in URL, use it; otherwise keep current or default to "/"
      if (query.path !== undefined && query.path !== null && query.path !== '') {
        this.filePath = String(query.path);
      } else if (query.path === '') {
        // Explicitly empty path in URL means reset
        this.filePath = "/";
      }

      if (query.source !== undefined && query.source !== null) {
        this.selectedSource = String(query.source);
      }

      if (query.interval !== undefined && query.interval !== null) {
        const parsed = parseInt(String(query.interval), 10);
        if (!isNaN(parsed) && parsed > 0) {
          // Validate interval based on permissions
          this.selectedInterval = this.validateInterval(parsed);
        }
      } else {
        // If no interval in query, validate current interval
        this.selectedInterval = this.validateInterval(this.selectedInterval);
      }

      if (query.lines !== undefined && query.lines !== null) {
        const parsed = parseInt(String(query.lines), 10);
        if (!isNaN(parsed) && parsed >= 1 && parsed <= 50) {
          this.selectedLines = parsed;
        }
      }
    },
    updateUrl() {
      this.$nextTick(() => {
        const query = {};

        if (this.filePath && this.filePath !== "/") {
          query.path = this.filePath;
        }

        if (this.selectedSource) {
          query.source = this.selectedSource;
        }

        if (this.selectedInterval !== 5) {
          query.interval = String(this.selectedInterval);
        }

        if (this.selectedLines !== 10) {
          query.lines = String(this.selectedLines);
        }

        const newQueryString = new URLSearchParams(query).toString();
        const currentQuery = this.$route.query || {};
        const currentQueryString = new URLSearchParams(
          Object.entries(currentQuery).reduce((acc, [key, value]) => {
            if (value !== null && value !== undefined) {
              acc[key] = String(value);
            }
            return acc;
          }, {})
        ).toString();

        if (newQueryString !== currentQueryString) {
          this.$router.replace({
            path: this.$route.path,
            query: Object.keys(query).length > 0 ? query : undefined,
          }).catch(() => {});
        }
      });
    },
    async toggleWatch() {
      if (this.watching) {
        this.stopWatch();
      } else {
        await this.startWatch();
      }
    },
    async startWatch() {
      if (!this.canStart) {
        return;
      }

      // Validate interval before starting - ensure it's valid for current user permissions
      this.selectedInterval = this.validateInterval(this.selectedInterval);

      this.watching = true;
      this.error = null;
      this.outputLines = [];
      this.currentLatency = 0;
      this.fileSize = null;
      this.fileName = null;
      this.fileModified = null;
      this.lastUpdateTime = null;

      // Check if user has realtime permissions - if so, always use SSE regardless of interval
      const realtimePerm = state.user?.permissions?.realtime;
      const hasRealtime = realtimePerm === true;
      const useRealtime = hasRealtime;

      this.useRealtime = useRealtime;

      if (useRealtime) {
        // Use dedicated SSE connection for realtime updates
        this.startSSEWatch();
      } else {
        // Use REST API polling
        await this.fetchFileContent(false);
        this.pollInterval = setInterval(() => {
          this.fetchFileContent(false);
        }, this.selectedInterval * 1000);
      }
    },
    stopWatch() {
      this.watching = false;
      if (this.pollInterval) {
        clearInterval(this.pollInterval);
        this.pollInterval = null;
      }
      if (this.eventSource) {
        this.eventSource.close();
        this.eventSource = null;
      }
      if (this.latencyPingInterval) {
        clearInterval(this.latencyPingInterval);
        this.latencyPingInterval = null;
      }
    },
    startSSEWatch() {
      // Build SSE URL with query parameters
      const params = new URLSearchParams({
        path: this.filePath,
        source: this.selectedSource,
        lines: this.selectedLines.toString(),
        interval: this.selectedInterval.toString(),
      });

      const sseUrl = getApiPath(`api/tools/watch/sse?${params.toString()}`);
      
      // Create EventSource connection
      this.eventSource = new EventSource(sseUrl);

      // Start latency pinging at the same interval as SSE events
      this.startLatencyPing();

      this.eventSource.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data);
          
          // Check if the data is wrapped in eventType/message format (from events system)
          let data = parsed;
          if (parsed.eventType === 'fileWatch' && parsed.message) {
            // The message is a JSON string that needs to be parsed
            data = typeof parsed.message === 'string' ? JSON.parse(parsed.message) : parsed.message;
          }
          
          // Handle connection status messages
          if (data.status) {
            if (data.status === 'shutdown') {
              this.stopWatch();
            } else if (data.status === 'error') {
              console.error('[FileWatcher] SSE error:', data.error);
              this.error = data.error || 'SSE connection error';
              this.stopWatch();
            }
            return;
          }

          // Handle file watch data
          this.handleFileWatchEvent(data);
        } catch (err) {
          console.error('[FileWatcher] Error parsing SSE event:', err, 'Raw data:', event.data);
          this.error = 'Failed to parse SSE event';
        }
      };

      this.eventSource.onerror = (error) => {
        console.error('[FileWatcher] SSE connection error:', error);
        this.error = 'SSE connection error';
        this.stopWatch();
      };
    },
    async fetchFileContent() {
      const startTime = Date.now();
      try {
        const params = {
          path: encodeURIComponent(this.filePath),
          source: encodeURIComponent(this.selectedSource),
          lines: this.selectedLines.toString(),
        };

        const apiPath = getApiPath("api/tools/watch", params);
        const res = await fetchURL(apiPath);
        const data = await res.json();

        const latency = Date.now() - startTime;
        this.currentLatency = latency;

        // Update last update time
        this.lastUpdateTime = new Date();

        // Update file metadata from response
        if (data.metadata) {
          if (data.metadata.name) {
            this.fileName = data.metadata.name;
          }
          if (data.metadata.size !== undefined) {
            this.fileSize = getHumanReadableFilesize(data.metadata.size);
          }
          if (data.metadata.modified) {
            this.fileModified = formatTimestamp(data.metadata.modified, state.user?.locale || 'en');
          }
        }

        // Log event with metadata
        console.log('[FileWatcher] Event received (REST)', {
          timestamp: new Date().toISOString(),
          latency: `${latency}ms`,
          isText: data.isText,
          metadata: data.metadata ? {
            name: data.metadata.name,
            size: data.metadata.size,
            type: data.metadata.type,
            modified: data.metadata.modified,
            path: data.metadata.path
          } : null,
          hasContents: !!data.contents,
          lineCount: data.isText && data.contents ? data.contents.split('\n').length : null
        });

        // Handle text files or metadata
        if (data.isText && data.contents) {
          // Text file - show content
          const lines = data.contents.split('\n');
          this.replaceOutputLines(lines, latency);
        } else if (!data.isText && data.metadata) {
          // Non-text file - show metadata
          const metadataLines = this.formatMetadata(data.metadata);
          this.replaceOutputLines(metadataLines, latency);
        }

        // Scroll to bottom
        this.$nextTick(() => {
          this.scrollToBottom();
        });
      } catch (err) {
        this.error = err.message || "Failed to fetch file content";
        this.stopWatch();
      }
    },
    handleFileWatchEvent(data) {
      // Update last update time
      this.lastUpdateTime = new Date();

      // Update file metadata from response
      if (data.metadata) {
        if (data.metadata.name) {
          this.fileName = data.metadata.name;
        }
        if (data.metadata.size !== undefined) {
          this.fileSize = getHumanReadableFilesize(data.metadata.size);
        }
        if (data.metadata.modified) {
          this.fileModified = formatTimestamp(data.metadata.modified, state.user?.locale || 'en');
        }
      }

      // Log event with metadata
      console.log('[FileWatcher] Event received', {
        isText: data.isText,
        metadata: data.metadata ? {
          name: data.metadata.name,
          size: data.metadata.size,
          type: data.metadata.type,
          modified: data.metadata.modified,
          path: data.metadata.path
        } : null,
        hasContents: !!(data.contents || data.content),
        lineCount: data.isText && (data.contents || data.content) ? (data.contents || data.content).split('\n').length : null
      });

      // Handle text files or metadata
      if (data.isText && (data.contents || data.content)) {
        // Text file - show content
        const content = data.contents || data.content;
        const lines = content.split('\n');
        this.replaceOutputLines(lines);
      } else if (!data.isText && data.metadata) {
        // Non-text file - show metadata
        const metadataLines = this.formatMetadata(data.metadata);
        this.replaceOutputLines(metadataLines);
      }

      // Scroll to bottom
      this.$nextTick(() => {
        this.scrollToBottom();
      });
    },
    formatMetadata(metadata) {
      // Format file metadata into display lines
      const lines = [];
      lines.push(`File: ${metadata.name}`);
      lines.push(`Path: ${metadata.path}`);
      lines.push(`Size: ${getHumanReadableFilesize(metadata.size)}`);
      lines.push(`Type: ${metadata.type || 'unknown'}`);
      if (metadata.modified) {
        lines.push(`Modified: ${formatTimestamp(metadata.modified, state.user?.locale || 'en')}`);
      }
      return lines;
    },
    replaceOutputLines(lines) {
      // Replace all output lines with new content
      this.outputLines = lines.map((line) => ({
        text: line,
        timestamp: Date.now(),
      }));

      // Keep only last 1000 lines to prevent memory issues
      if (this.outputLines.length > 1000) {
        this.outputLines = this.outputLines.slice(-1000);
      }
    },
    scrollToBottom() {
      const terminal = this.$refs.terminalOutput;
      if (terminal) {
        terminal.scrollTop = terminal.scrollHeight;
      }
    },
    startLatencyPing() {
      // Clear any existing interval
      if (this.latencyPingInterval) {
        clearInterval(this.latencyPingInterval);
      }

      // Ping health endpoint immediately
      this.pingHealthEndpoint();

      // Start pinging at the same interval as SSE events
      this.latencyPingInterval = setInterval(() => {
        this.pingHealthEndpoint();
      }, this.selectedInterval * 1000);
    },
    async pingHealthEndpoint() {
      const startTime = Date.now();
      try {
        const params = {
          latencyCheck: "true",
        };
        const apiPath = getApiPath("api/tools/watch", params);
        await fetchURL(apiPath);
        const roundTripLatency = Date.now() - startTime;
        // Use half of round-trip latency as estimate for one-way latency
        this.currentLatency = Math.round(roundTripLatency / 2);
      } catch (err) {
        console.error('[FileWatcher] Latency ping failed:', err);
        // Don't update latency on error, keep previous value
      }
    },
    // Spacebar key shortcut to toggle watch
    handleKeydown(event) {
      if (event.keyCode === 32 || event.key === ' ') {
        // Don't trigger if we are typing, since someone can be editing a Link in the sidebar
        const activeElement = document.activeElement;
        const isInputFocused = activeElement && (activeElement.tagName === 'INPUT' || activeElement.tagName === 'TEXTAREA');
        if (!isInputFocused) {
          this.toggleWatch();
        }
      }
    },
  },
};
</script>

<style scoped>
.file-watcher {
  padding: 2rem;
  width: 100%;
  margin: 0 auto;
}

.file-watcher-config {
  margin-bottom: 2em;
}

.searchContext {
  margin-bottom: 0 !important;
}

.config-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: nowrap;
  padding: 1em !important;
}

.config-row.mobile {
  flex-direction: column;
  align-items: stretch;
}

.config-row-second {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex: 1;
}

.config-row-second.mobile {
  width: 100%;
  flex-direction: row;
}

.config-item {
  display: flex;
  align-items: center;
}

.config-item.lines-input {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
}

.lines-label {
  font-size: 0.875rem;
  color: var(--textSecondary);
  white-space: nowrap;
  margin: 0;
  flex-shrink: 0;
}

.file-picker-button {
  width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-picker {
  flex-grow: 1;
}

.config-row.mobile .file-picker {
  width: 100%;
}


.file-watcher-output {
  width: 100%;
  padding: 0 !important;
}

.file-picker-button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.error-message {
  background: #fee;
  color: #c33;
  padding: 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
  border: 1px solid #fcc;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--textSecondary);
  font-size: 0.9rem;
}

.terminal-header {
  display: flex;
  flex-direction: column;
  padding: 0.75rem 1rem;
  background: var(--surfaceSecondary, rgba(0, 0, 0, 0.05));
  border-bottom: 1px solid var(--borderPrimary, rgba(0, 0, 0, 0.1));
  margin-bottom: 0;
  font-size: 0.9rem;
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
  gap: 0.5rem;
}

.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: relative;
}

.header-row-first {
  min-height: 1.5rem;
}

.header-row-second {
  min-height: 1.5rem;
  padding-top: 0.25rem;
}

.header-row-second-mobile {
  min-height: 1.5rem;
  padding-top: 0.25rem;
  justify-content: center;
}

.header-row-third-mobile {
  min-height: 1.5rem;
  padding-top: 0.25rem;
}

.header-center-mobile {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  justify-content: center;
  width: 100%;
}

.header-left,
.header-center,
.header-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.header-center {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  white-space: nowrap;
}

.header-right {
  justify-content: flex-end;
  margin-left: auto;
}

.header-filename {
  font-weight: 700;
  color: var(--textPrimary);
}

.header-filesize {
  color: var(--textSecondary);
}

.header-label {
  color: var(--textSecondary);
  font-weight: 500;
}

.header-value {
  color: var(--textPrimary);
  font-weight: 600;
}

.header-value.latency-good {
  color: #4caf50; /* Green */
}

.header-value.latency-ok {
  color: #ff9800; /* Yellow/Orange */
}

.header-value.latency-slow {
  color: #f44336; /* Red */
}

.header-value.latency-inactive {
  color: var(--textSecondary); /* Gray */
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #4caf50;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.terminal-output {
  font-family: 'Courier New', 'Consolas', 'Monaco', monospace;
  font-size: 14px;
  padding: 1rem;
  min-height: 100px;
  overflow-y: auto;
  overflow-x: auto;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
  /* Dark mode (default) */
  background: #1e1e1e;
  color: #d4d4d4;
}

.terminal-output:not(.dark-mode) {
  /* Light mode */
  background: #ffffff;
  color: #1e1e1e;
  border: 1px solid var(--borderPrimary, rgba(0, 0, 0, 0.1));
}

.terminal-line {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin-bottom: 2px;
  line-height: 1.5;
  white-space: nowrap;
}

.terminal-text {
  white-space: nowrap;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--textSecondary);
}

.empty-state .material-icons {
  font-size: 4rem;
  opacity: 0.3;
  margin-bottom: 1rem;
}

.empty-state p {
  font-size: 1.1rem;
}

@media (max-width: 768px) {
  .file-watcher {
    padding: 1rem;
  }

  .config-row {
    flex-direction: column;
    align-items: stretch;
    gap: 0.75rem;
  }

  .config-item.file-picker {
    width: 100% !important;
  }

  .config-item.file-picker .file-picker-button {
    width: 100%;
  }

  .config-row-second {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
  }

  .config-item.interval-select,
  .config-item.lines-input,
  .config-item.play-button {
    flex: 1;
    width: auto !important;
  }

  .terminal-output {
    font-size: 12px;
    min-height: 100px;
  }
}
</style>

