<template>
  <div class="error-message">
    <h2 class="message">
      <i class="material-symbols">{{ info.icon }}</i>
      <span>{{ errorMessage }}</span>
    </h2>
  </div>
</template>

<script>
const errors = {
  0: { icon: "cloud_off" },
  401: { icon: "error" },
  403: { icon: "error" },
  404: { icon: "gps_off" },
  share404: { icon: "gps_off" },
  500: { icon: "error_outline" },
};

export default {
  name: "errors",
  props: ["errorCode", "showHeader"],
  computed: {
    info() {
      return errors[this.errorCode] || errors[500];
    },
    errorMessage() {
      switch (this.errorCode) {
        case 0:
          return this.$t('errors.connection');
        case 401:
        case 403:
          return this.$t('errors.forbidden');
        case 404:
          return this.$t('errors.notFound');
        case 'share404':
          return this.$t('errors.shareNotFound');
        default:
          return this.$t('errors.internal');
      }
    },
  },
};
</script>
