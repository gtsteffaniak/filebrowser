<template>
    <div class="settings-group">
        <div class="settings-group-title button" :class="{ 'unclickable': !collapsable }"
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
export default {
  name: 'SettingsItem',
  props: {
    title: {
      type: String,
      required: true
    },
    collapsable: {
      type: Boolean,
      default: false
    },
    startCollapsed: {
      type: Boolean,
      default: false
    },
    forceCollapsed: {
      type: Boolean,
      default: null
    }
  },
  data() {
    return {
      isCollapsed: this.startCollapsed
    }
  },
  computed: {
    actuallyCollapsed() {
      // If forceCollapsed is explicitly set, use that, otherwise use internal state
      return this.forceCollapsed !== null ? this.forceCollapsed : this.isCollapsed;
    }
  },
  watch: {
    forceCollapsed(newVal) {
      if (newVal !== null) {
        this.isCollapsed = newVal;
      }
    }
  },
  methods: {
    toggleCollapse() {
      if (this.forceCollapsed !== null) {
        // Emit event for parent to handle accordion logic
        this.$emit('toggle', this.title);
      } else {
        // Handle internally as before
        this.isCollapsed = !this.isCollapsed;
      }
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
      element.style.height = element.scrollHeight + 'px';
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
      element.style.height = element.scrollHeight + 'px';
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
    margin-top: 1em;
    background: var(--alt-background) !important;
    color: var(--textPrimary) !important;
    padding: 0.5em;
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
