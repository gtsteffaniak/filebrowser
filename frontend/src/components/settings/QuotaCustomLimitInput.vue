<template>
  <div class="form-flex-group quota-custom-limit-input">
    <input
      class="input form-grow flat-right"
      type="number"
      min="1"
      :value="amount"
      :aria-label="ariaLabel"
      :disabled="disabled"
      @input="onAmountInput"
    />
    <ExpandDropdown
      :model-value="unit"
      class="flat-left form-compact form-dropdown quota-unit-dropdown"
      :options="unitOptions"
      :aria-label="ariaLabel"
      :disabled="disabled"
      @update:model-value="onUnitChange"
    />
  </div>
</template>

<script>
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";

export default {
  name: "QuotaCustomLimitInput",
  components: { ExpandDropdown },
  props: {
    amount: { type: Number, default: 1 },
    unit: { type: String, default: "gb" },
    ariaLabel: { type: String, default: "" },
    disabled: { type: Boolean, default: false },
  },
  emits: ["update:amount", "update:unit"],
  computed: {
    unitOptions() {
      return [
        { value: "mb", label: "MB" },
        { value: "gb", label: "GB" },
      ];
    },
  },
  methods: {
    onAmountInput(event) {
      const value = Number(event.target.value);
      this.$emit("update:amount", Number.isFinite(value) ? value : 1);
    },
    onUnitChange(value) {
      this.$emit("update:unit", value === "mb" ? "mb" : "gb");
    },
  },
};
</script>

<style scoped>
.quota-custom-limit-input {
  width: 100%;
}

:global(.expand-dropdown.form-dropdown.quota-unit-dropdown) {
  width: 6.5rem;
  min-width: 6.5rem;
}

</style>
