<template>
    <div class="settings-group">
        <div v-if="!hidden" class="settings-group-title button" :class="{ 'unclickable': !collapsable }"
            @click="collapsable ? toggleCollapse() : null">
            <h3>{{ title }}</h3>
            <i v-if="collapsable" class="material-symbols-outlined collapse-icon" :class="{ 'rotated': !actuallyCollapsed }">
                expand_more
            </i>
        </div>

        <transition :css="false" @enter="collapseEnter" @leave="collapseLeave">
            <div v-show="!actuallyCollapsed" class="settings-content">
                <slot></slot>
            </div>
        </transition>
    </div>
</template>

<script>
import { SETTINGS_ACCORDION_KEY } from "./settingsAccordion.js";
import { collapseEnter, collapseLeave } from "@/utils/transitions.ts";

export default {
  name: 'SettingsItem',
  inject: {
    settingsAccordion: {
      from: SETTINGS_ACCORDION_KEY,
      default: null,
    },
  },
  props: {
    title: {
      type: String,
      required: true
    },
    collapsable: {
      type: Boolean,
      default: false
    },
    hidden: {
      type: Boolean,
      default: false
    },
    startCollapsed: {
      type: Boolean,
      default: false
    },
    /** When true, only one section in a parent SettingsAccordion may be open at a time. */
    accordion: {
      type: Boolean,
      default: false,
    },
    /** Section id within a SettingsAccordion; required when accordion is true. */
    name: {
      type: String,
      default: null,
    },
  },
  emits: {
    toggled: (state) => typeof state === 'string' || typeof state === 'boolean',
  },
  data() {
    return {
      isCollapsed: this.startCollapsed
    }
  },
  computed: {
    inAccordion() {
      return this.accordion && this.settingsAccordion && this.name;
    },
    actuallyCollapsed() {
      if (this.inAccordion) {
        return this.settingsAccordion.expanded() !== this.name;
      }
      return this.isCollapsed;
    }
  },
  watch: {
    startCollapsed(newVal) {
      if (!this.inAccordion) {
        this.isCollapsed = newVal;
      }
    },
  },
  methods: {
    toggleCollapse() {
      if (this.inAccordion) {
        this.settingsAccordion.toggle(this.name);
        this.$emit('toggled', this.name);
        return;
      }
      this.isCollapsed = !this.isCollapsed;
      this.$emit('toggled', !this.isCollapsed);
    },
    collapseEnter,
    collapseLeave,
  }
}
</script>

<style scoped>


.settings-group-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--alt-background) !important;
    color: var(--textPrimary) !important;
    padding: 0.5em;
    margin-top: 0.75em;
}

.settings-group-title h3 {
    margin: 0;
    flex: 1;
}

.collapse-icon {
    transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    color: var(--textSecondary);
}

.collapse-icon.rotated {
    transform: rotate(180deg);
}

.settings-content {
  overflow: hidden;
  margin-top: 0.5em;
  padding: 0 0.5em 1em;
}

.settings-group-title.unclickable {
    cursor: default;
    user-select: none;
    transition: opacity 0.2s ease;
}
</style>
