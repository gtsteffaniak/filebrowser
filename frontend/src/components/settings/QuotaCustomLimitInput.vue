<template>
  <div class="form-flex-group quota-custom-limit-input">
    <input
      class="input form-grow flat-right"
      type="number"
      min="1"
      :value="amount"
      :aria-label="ariaLabel"
      @input="onAmountInput"
    />
    <ExpandDropdown
      :model-value="unit"
      class="flat-left form-compact form-dropdown"
      :options="unitOptions"
      :aria-label="ariaLabel"
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
