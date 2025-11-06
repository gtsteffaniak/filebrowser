<template>
  <div class="card-title">
    <h2>{{ $t("general.help") }}</h2>
  </div>

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
        <div class="shortcut-item">
          <kbd>/</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.slash") }}</span>
        </div>
      </div>

      <!-- Mouse Actions Section -->
      <div class="shortcut-section">
        <h3 class="section-title">{{ $t("help.mouseActions") }}</h3>
        <div class="shortcut-item">
          <kbd>Click</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.click") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + Click</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.click") }}</span>
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
          <kbd>CTRL + B</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.b") }}</span>
        </div>
        <div class="shortcut-item">
          <kbd>CTRL + ,</kbd> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <span>{{ $t("help.ctrl.comma") }}</span>
        </div>
      </div>
    </div>
  </div>

  <div class="card-action">
    <button type="submit" @click="closeHovers" class="button button--flat" :aria-label="$t('general.ok')"
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
    closeHovers() {
      return mutations.closeHovers; // Return the closeHovers mutation
    },
  },
};
</script>

<style scoped>
.help-content {
  max-height: 70vh;
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
