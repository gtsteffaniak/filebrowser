<template>
  <div class="card-content">
    <p class="file-summary">
      <i class="material-symbols-outlined file-icon">analytics</i>
      <span class="file-name">{{ item?.name }}</span>
    </p>

    <div class="props-grid">
      <div v-if="storedMass" class="stored-banner">
        {{ $t("threejs.propStored") }}
        <span v-if="storedConfiguration"> · {{ storedConfiguration }}</span>
      </div>
      <div class="prop-row">
        <span>{{ $t("threejs.propVolume") }}</span>
        <b>{{ storedMass ? fmtVolumeM3(storedMass.volumeM3) : fmtVolume(propsData.volume) }}</b>
      </div>
      <div class="prop-row">
        <span>{{ $t("threejs.propArea") }}</span>
        <b>{{ storedMass ? fmtAreaM2(storedMass.areaM2) : fmtArea(propsData.area) }}</b>
      </div>
      <div class="prop-row">
        <span>{{ $t("threejs.propMass") }}</span>
        <b>{{ storedMass ? fmtMassKg(storedMass.massKg) : mass }}</b>
      </div>
      <div v-if="storedMass" class="prop-row">
        <span>{{ $t("threejs.propCenter") }}</span>
        <b>{{ fmtPointM(storedMass.centerM) }}</b>
      </div>
      <div v-if="hasStoredInertia" class="prop-row">
        <span>{{ $t("threejs.propInertia") }}</span>
          <b>{{ storedMass.inertiaKgM2.map(v => Number(v).toExponential(3)).join(", ") }} kg·m²</b>
      </div>
      <div v-if="storedMass" class="prop-row">
        <span>{{ $t("threejs.propMaterial") }}</span>
        <b>{{ storedMaterial || "—" }}</b>
      </div>
      <div v-if="!storedMass" class="prop-row prop-inline">
        <span>{{ $t("threejs.propDensity") }}</span>
        <select v-model="density" class="density-select">
          <option v-for="m in materials" :key="m.name" :value="m.density">
            {{ m.name }} ({{ m.density }} g/cm³)
          </option>
          <option :value="customDensity">{{ $t("threejs.propCustom") }}</option>
        </select>
        <input
          v-if="density === customDensity"
          v-model.number="customValue"
          type="number"
          min="0.01"
          step="0.01"
          class="density-input"
        />
      </div>
      <div v-if="!storedMass" class="prop-row">
        <span>{{ $t("threejs.propCenter") }}</span>
        <b>{{ fmtPoint(propsData.center) }}</b>
      </div>
      <div class="prop-row">
        <span>{{ $t("threejs.propBBox") }}</span>
        <b>{{ propsData.bbox }}</b>
      </div>
      <div class="prop-row">
        <span>{{ $t("threejs.propTriangles") }}</span>
        <b>{{ propsData.triangles?.toLocaleString() }}</b>
      </div>
      <div class="prop-row">
        <span>{{ $t("threejs.propFaces") }}</span>
        <b>{{ propsData.faces?.toLocaleString() }}</b>
      </div>
    </div>

    <p class="props-note">{{ $t("threejs.propNote") }}</p>
  </div>
</template>

<script>
const CUSTOM = -1;

export default {
  name: "CadProperties",
  props: {
    item: { type: Object, required: true },
    propsData: { type: Object, required: true },
    storedProperties: { type: Array, default: () => [] },
  },
  data() {
    return {
      customDensity: CUSTOM,
      customValue: 1.0,
      density: 1.05,
      materials: [
        { name: "ABS", density: 1.05 },
        { name: "PLA", density: 1.24 },
        { name: "PETG", density: 1.27 },
        { name: "Nylon PA6", density: 1.14 },
        { name: "Aluminio", density: 2.70 },
        { name: "Acero", density: 7.85 },
        { name: "Inox 304", density: 8.00 },
        { name: "Latón", density: 8.50 },
      ],
    };
  },
  computed: {
    storedEntry() {
      return this.storedProperties.find((p) => p.available && p.mass) || null;
    },
    storedMass() {
      return this.storedEntry?.mass || null;
    },
    storedConfiguration() {
      return this.storedEntry?.configuration || "";
    },
    storedMaterial() {
      if (!this.storedEntry) return "";
      return [this.storedEntry.materialBase, this.storedEntry.material].filter(Boolean).join(" | ");
    },
    hasStoredInertia() {
      return Array.isArray(this.storedMass?.inertiaKgM2) && this.storedMass.inertiaKgM2.length > 0;
    },
    effectiveDensity() {
      return this.density === CUSTOM
        ? (Number(this.customValue) > 0 ? Number(this.customValue) : 0)
        : this.density;
    },
    // volume is in mm³; density in g/cm³ → g = mm³ × g/cm³ / 1000
    mass() {
      const grams = (this.propsData.volume || 0) * this.effectiveDensity / 1000;
      if (grams >= 1000) return `${(grams / 1000).toFixed(3)} kg`;
      return `${grams.toFixed(2)} g`;
    },
  },
  methods: {
    fmtVolumeM3(m3) {
      if (m3 == null) return "—";
      return `${(m3 * 1e6).toFixed(2)} cm³ (${(m3 * 1e9).toFixed(0)} mm³)`;
    },
    fmtAreaM2(m2) {
      if (m2 == null) return "—";
      return `${(m2 * 1e4).toFixed(2)} cm² (${(m2 * 1e6).toFixed(0)} mm²)`;
    },
    fmtMassKg(kg) {
      if (kg == null) return "—";
      return kg >= 1 ? `${kg.toFixed(4)} kg` : `${(kg * 1000).toFixed(2)} g`;
    },
    fmtPointM(point) {
      if (!point) return "—";
      return `(${point.map((v) => (v * 1000).toFixed(2)).join(", ")}) mm`;
    },
    fmtVolume(mm3) {
      if (mm3 == null) return "—";
      return `${(mm3 / 1000).toFixed(2)} cm³ (${mm3.toFixed(0)} mm³)`;
    },
    fmtArea(mm2) {
      if (mm2 == null) return "—";
      return `${(mm2 / 100).toFixed(2)} cm² (${mm2.toFixed(0)} mm²)`;
    },
    fmtPoint(p) {
      if (!p) return "—";
      return `(${p.map((v) => v.toFixed(2)).join(", ")}) mm`;
    },
  },
};
</script>

<style scoped>
.file-summary {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 1rem;
  font-weight: 600;
}

.file-icon {
  font-size: 28px;
  color: var(--altTextSecondary, #5f6368);
}

.stored-banner {
  margin-bottom: 0.65rem;
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  color: var(--green, #2e7d32);
  background: rgba(46, 125, 50, 0.12);
  font-size: 0.8rem;
  font-weight: 600;
}

.props-grid {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.prop-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.35rem 0.55rem;
  border-radius: 6px;
  background: var(--altBackground, rgba(127, 127, 127, 0.1));
  font-size: 0.88rem;
}

.prop-row b {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.prop-inline {
  flex-wrap: wrap;
}

.density-select {
  padding: 2px 6px;
  border-radius: 5px;
  border: 1px solid var(--border, #888);
  background: var(--background, #fff);
  color: inherit;
  font-size: 0.82rem;
}

.density-input {
  width: 5.5rem;
  padding: 2px 6px;
  border-radius: 5px;
  border: 1px solid var(--border, #888);
  background: var(--background, #fff);
  color: inherit;
  font-size: 0.82rem;
}

.props-note {
  margin-top: 0.9rem;
  font-size: 0.75rem;
  opacity: 0.65;
}
</style>
