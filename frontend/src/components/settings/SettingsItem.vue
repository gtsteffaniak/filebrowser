<template>
    <div class="settings-group">
        <div v-if="!hidden" class="settings-group-title button" :class="{ 'unclickable': !collapsable }"
            @click="collapsable ? toggleCollapse() : null">
            <h3>{{ title }}</h3>
            <i v-if="collapsable" class="material-symbols-outlined collapse-icon" :class="{ 'rotated': !actuallyCollapsed }">
                expand_more
            </i>
        </div>

        <transition name="expand" @before-enter="beforeEnter" @enter="enter" @leave="leave">
            <div v-show="!actuallyCollapsed" class="settings-content">
                <slot></slot>
            </div>
        </transition>
    </div>
</template>

<script>
import { SETTINGS_ACCORDION_KEY } from "./settingsAccordion.js";

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
  emits: ['toggle'],
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
        this.$emit('toggle', this.name);
        return;
      }
      this.isCollapsed = !this.isCollapsed;
      this.$emit('toggle', !this.isCollapsed);
    },
    /**
     * @param {Element} el
     */
    beforeEnter(el) {
      const element = /** @type {HTMLElement} */ (el);
      element.style.height = '0';
      element.style.opacity = '0';
    },
    /**
     * @param {Element} el
     * @param {() => void} done
     */
    enter(el, done) {
      const element = /** @type {HTMLElement} */ (el);
      element.style.transition = '';
      element.style.height = '0';
      element.style.opacity = '0';
      void element.offsetHeight;
      element.style.transition = 'height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1)';
      element.style.height = `${element.scrollHeight}px`;
      element.style.opacity = '1';
      setTimeout(() => {
        element.style.height = 'auto';
        done();
      }, 300);
    },
    /**
     * @param {Element} el
     * @param {() => void} done
     */
    leave(el, done) {
      const element = /** @type {HTMLElement} */ (el);
      element.style.transition = 'height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1)';
      element.style.height = `${element.scrollHeight}px`;
      void element.offsetHeight;
      element.style.height = '0';
      element.style.opacity = '0';
      setTimeout(done, 300);
    },
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
    /* Room for ExpandDropdown box-shadow (clipped otherwise on last items) */
    padding: 0 0.5em 1em;
}

.expand-enter-active,
.expand-leave-active {
    transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    overflow: hidden;
}

.expand-enter,
.expand-leave-to {
    height: 0 !important;
    opacity: 0;
}

.settings-group-title.unclickable {
    cursor: default;
    user-select: none;
    transition: opacity 0.2s ease;
}

/* Prevent content height issues during animation */
.settings-content .input,
.settings-content .settings-items {
    height: auto;
}
</style>
