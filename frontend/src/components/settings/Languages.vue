<template>
  <select @change="change" :value="locale">
    <option v-for="(value, label) in locales" :key="label" :value="label">
      {{ $t("languages." + label) }}
    </option>
  </select>
</template>

<script>
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
    change(event) {
      const target = event.target;
      this.$emit("update:locale", target.value);
    },
  },
});
</script>
