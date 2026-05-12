<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
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
          <kbd>F1</kbd>
          <span>{{ $t("help.f1") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>F2</kbd>
          <span>{{ $t("help.f2") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>DEL</kbd>
          <span>{{ $t("help.del") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>ESC</kbd>
          <span>{{ $t("help.esc") }}</span>
        </div>
      </div>

      <!-- Mouse Actions Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.mouseActions") }}</h3>
        <div class="shortcut-item">
          <kbd>Click</kbd>
          <span>{{ $t("help.click") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>Double click</kbd>
          <span>{{ $t("help.dbClick") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>Click + Drag</kbd>
          <span>{{ $t("help.marqueeSelect") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + Click</kbd>
          <span>{{ $t("help.ctrl.click") }}</span>
        </div>
      </div>

      <!-- File Operations Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.fileOperations") }}</h3>
        <div class="shortcut-item">
          <kbd>CTRL + C</kbd>
          <span>{{ $t("help.ctrl.c") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + X</kbd>
          <span>{{ $t("help.ctrl.x") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + V</kbd>
          <span>{{ $t("help.ctrl.v") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + D</kbd>
          <span>{{ $t("help.ctrl.d") }}</span>
        </div>
      </div>

      <!-- Navigation Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("general.navigation") }}</h3>
        <div class="shortcut-item">
          <kbd>CTRL + B</kbd>
          <span>{{ $t("help.ctrl.b") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + ,</kbd>
          <span>{{ $t("help.ctrl.comma") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>space or /</kbd>
          <span>{{ $t("help.slash") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + E</kbd>
          <span>{{ $t("help.ctrl.e") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + F1</kbd>
          <span>{{ $t("help.ctrl.f1") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + F2</kbd>
          <span>{{ $t("help.ctrl.f2") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + F3</kbd>
          <span>{{ $t("help.ctrl.f3") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + Scroll</kbd>
          <span>{{ $t("help.ctrl.scroll") }}</span>
        </div>
      </div>

      <!-- Media Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.media.title") }}</h3>
        <div class="shortcut-item">
          <kbd>Space / K</kbd>
          <span>{{ $t("help.media.playPause") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>1 to 9</kbd>
          <span>{{ $t("help.media.seekPercentage") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>← / →</kbd>
          <span>{{ $t("help.media.seek") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>↑ / ↓</kbd>
          <span>{{ $t("help.media.volume") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>Q</kbd>
          <span>{{ $t("help.media.playbackQueue") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>E</kbd>
          <span>{{ $t("help.media.switchTabs") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>L</kbd>
          <span>{{ $t("help.media.loop") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>P</kbd>
          <span>{{ $t("help.media.playbackModes") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>F</kbd>
          <span>{{ $t("help.media.fullscreen") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>M</kbd>
          <span>{{ $t("help.media.mute") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>C</kbd>
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
