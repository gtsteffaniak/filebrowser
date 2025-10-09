<template>
  <div class="card-title">
    <h2>{{ $t("help.help") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("help.description") }} <a class="link" href="https://github.com/gtsteffaniak/filebrowser/wiki" target="_blank">{{ $t("help.wiki") }}</a></p>
    <ul>
      <li><strong>F1</strong> - {{ $t("help.f1") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>F2</strong> - {{ $t("help.f2") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>DEL</strong> - {{ $t("help.del") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>ESC</strong> - {{ $t("help.esc") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>CTRL + Click</strong> - {{ $t("help.ctrl.click") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>Click</strong> - {{ $t("help.click") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>CTRL + D</strong> - {{ $t("help.ctrl.d") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>CTRL + B</strong> - {{ $t("help.ctrl.b") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>CTRL + ,</strong> - {{ $t("help.ctrl.comma") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <li><strong>/</strong> - {{ $t("help.slash") }}</li> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </ul>
  </div>

  <div class="card-action">
    <button type="submit" @click="closeHovers" class="button button--flat" :aria-label="$t('buttons.ok')"
      :title="$t('buttons.ok')">
      {{ $t("buttons.ok") }}
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
