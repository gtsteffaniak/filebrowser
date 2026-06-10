<template>
  <div class="card-content help-content">
    <p class="help-description">
      {{ $t("help.description") }}
      <a class="link" href="https://filebrowserquantum.com/" target="_blank">
        {{ $t("help.wiki") }}
      </a>
    </p>

    <div class="shortcuts-grid">
      <!-- Function Keys Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.functionKeys") }}</h3>
        <div class="shortcut-item">
          <kbd>F1</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.f1") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>F2</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.f2") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>DEL</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.del") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>ESC</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.esc") }}</span>
        </div>
      </div>

      <!-- Mouse Actions Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.mouseActions") }}</h3>
        <div class="shortcut-item">
          <kbd>CLICK</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.click") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>DOUBLE CLICK</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.dbClick") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CLICK + DRAG</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.marqueeSelect") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + CLICK</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.click") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + SCROLL</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.scroll") }}</span>
        </div>
      </div>

      <!-- File Operations Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.fileOperations") }}</h3>
        <div class="shortcut-item">
          <kbd>CTRL + C</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.c") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + X</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.x") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + V</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.v") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + D</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.d") }}</span>
        </div>

      </div>

      <!-- Navigation Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("general.navigation") }}</h3>
        <div class="shortcut-item">
          <kbd>BACKSPACE {{ $t("general.or") }} ALT + ↑</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.altArrowUp") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>ENTER {{ $t("general.or") }} ALT + ↓</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.altArrowDown") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + B</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.b") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + ,</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.comma") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>SPACE {{ $t("general.or") }} /</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.slash") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + E</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.e") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + F1</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.f1") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + F2</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.f2") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + F3</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.f3") }}</span>
        </div>
      </div>

      <!-- Media Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.media.title") }}</h3>
        <div class="shortcut-item">
          <kbd>SPACE / K</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.playPause") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>1 {{ $t("general.to") }} 9</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.seekPercentage") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>← / →</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.seek") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>↑ / ↓</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.volume") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>Q</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.playbackQueue") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>E</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.switchTabs") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>L</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.loop") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>P</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.playbackModes") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>F</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.fullscreen") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>M</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.mute") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>C</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.media.captions") }}</span>
        </div>
      </div>
    </div>
  </div>

  <div class="card-actions">
    <button type="submit" @click="closeTopPrompt" class="button button--flat" :aria-label="$t('general.ok')"
      :title="$t('general.ok')">
      {{ $t("general.ok") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store"; // Import the mutations

export default {
  name: "help",
  computed: {
    closeTopPrompt() {
      return mutations.closeTopPrompt();
    },
  },
};
</script>

<style scoped>
.help-content {
  overflow-y: auto;
}

.help-description {
  margin-bottom: 1.5em;
  color: var(--textSecondary);
  line-height: 1.5;
}

.shortcuts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1.5em;
}

.shortcut-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.section-title {
  font-size: 0.95em;
  font-weight: 600;
  color: var(--textPrimary);
  margin: 0 0 0.75em 0;
  padding-bottom: 0.5em;
  border-bottom: 1px solid var(--divider);
}

.shortcut-item {
  display: flex;
  align-items: center;
  gap: 0.75em;
  padding: 0.5em;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.shortcut-item:hover {
  background-color: var(--surfaceSecondary);
}

.shortcut-item kbd {
  display: inline-block;
  min-width: 80px;
  padding: 0.35em 0.65em;
  font-family: monospace;
  font-size: 0.85em;
  font-weight: 600;
  line-height: 1.2;
  color: var(--textPrimary);
  background-color: var(--surfaceSecondary);
  border: 1px solid var(--divider);
  border-radius: 4px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  text-align: center;
}

.shortcut-item span {
  flex: 1;
  font-size: 0.9em;
  color: var(--textSecondary);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .shortcuts-grid {
    grid-template-columns: 1fr;
  }

  .shortcut-item kbd {
    min-width: 70px;
    font-size: 0.8em;
  }
}
</style>
