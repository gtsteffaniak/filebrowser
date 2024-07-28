<template>
  <select v-on:change="change" :value="locale">
    <option v-for="(label, value) in locales" :key="value" :value="value">
      {{ $t("languages." + label) }}
    </option>
  </select>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
  name: "Languages",
  props: {
    locale: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      locales: {
        he: "he",
        hu: "hu",
        ar: "ar",
        de: "de",
        en: "en",
        es: "es",
        fr: "fr",
        is: "is",
        it: "it",
        ja: "ja",
        ko: "ko",
        "nl-be": "nlBE",
        pl: "pl",
        "pt-br": "ptBR",
        pt: "pt",
        ro: "ro",
        ru: "ru",
        sk: "sk",
        "sv-se": "svSE",
        tr: "tr",
        ua: "ua",
        "zh-cn": "zhCN",
        "zh-tw": "zhTW",
      },
    };
  },
  methods: {
    change(event: Event) {
      const target = event.target as HTMLSelectElement;
      this.$emit("update:locale", target.value);
    },
  },
});
</script>
