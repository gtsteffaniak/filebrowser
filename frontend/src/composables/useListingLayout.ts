import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";
import { state, getters } from "@/store/index.ts";
import throttle from "@/utils/throttle.js";

export function useListingLayout(options: { container?: Ref<HTMLElement | null> } = {}) {
  const columnWidth = ref(250 + state.user.gallerySize * 50);
  const width = ref(window.innerWidth);

  const numColumns = computed(() => {
    void width.value;
    if (!getters.isCardView()) {
      return 1;
    }
    const elem = document.querySelector<HTMLElement>("#main");
    if (!elem) {
      return 1;
    }
    if (getters.viewMode() === "icons") {
      const containerSize = 70 + state.user.gallerySize * 15;
      let columns = Math.floor(elem.offsetWidth / containerSize);
      if (columns === 0) {
        columns = 1;
      }
      const minColumns = 3;
      const maxColumns = 12;
      return Math.max(minColumns, Math.min(columns, maxColumns));
    }
    let columns = Math.floor(elem.offsetWidth / columnWidth.value);
    if (columns === 0) {
      columns = 1;
    }
    return columns;
  });

  const itemStyles = computed(() => {
    const viewMode = getters.viewMode();
    const styles: Record<string, string> = {};
    const size = state.user.gallerySize;

    if (viewMode === "icons") {
      const baseSize = 20 + (size * 15); // 35px to 155px
      const cellSize = baseSize + 30;
      styles["--icon-size"] = `${baseSize}px`;
      styles["--icon-font-size"] = `${baseSize}px`;
      styles["--icons-view-cell-size"] = `${cellSize}px`;
    } else if (viewMode === "gallery") {
      const baseCalc = 80 + (size * 25);
      const extraScaling = Math.max(0, size - 5) * 15;
      const baseSize = baseCalc + extraScaling; // Size 5: 205px, Size 9: 345px
      const iconFontSize = (3 + (size * 0.5)).toFixed(2); // 3em to 7.5em
      styles["--icon-font-size"] = `${iconFontSize}em`;

      if (getters.isMobile()) {
        const minWidth = size <= 3 ? 120 : size <= 7 ? 160 : 280;
        const mobileHeight = 120 + (size * 20); // 120px to 300px
        styles["--gallery-mobile-min-width"] = `${minWidth}px`;
        styles["--item-width"] = `${minWidth}px`;
        styles["--item-height"] = `${mobileHeight}px`;
      } else {
        styles["--item-width"] = `${baseSize}px`;
        styles["--item-height"] = `${Math.round(baseSize * 1.2)}px`;
      }
    } else if (viewMode === "list" || viewMode === "compact") {
      const baseHeight = viewMode === "compact"
        ? 40 + (size * 2)  // 42px to 58px
        : 50 + (size * 3); // 53px to 77px
      const iconSize = (2 + (size * 0.12)).toFixed(2); // 2.12em to 3.08em
      const iconFontSize = (1.5 + (size * 0.12)).toFixed(2); // 1.62em to 2.58em

      styles["--item-width"] = `calc(${(100 / numColumns.value).toFixed(2)}% - 1em)`;
      styles["--item-height"] = `${baseHeight}px`;
      styles["--icon-size"] = `${iconSize}em`;
      styles["--icon-font-size"] = `${iconFontSize}em`;
    } else {
      // Normal view
      const iconSize = (3.2 + (size * 0.15)).toFixed(2); // 3.35em to 4.55em
      const iconFontSize = (2.2 + (size * 0.12)).toFixed(2); // 2.32em to 3.28em

      styles["--item-width"] = `calc(${(100 / numColumns.value)}% - 1em)`;
      styles["--item-height"] = "auto";
      styles["--icon-size"] = `${iconSize}em`;
      styles["--icon-font-size"] = `${iconFontSize}em`;
    }

    return styles;
  });

  watch(
    () => state.user.gallerySize,
    () => {
      columnWidth.value = 250 + state.user.gallerySize * 50;
    },
  );

  let resizeTimeout: ReturnType<typeof setTimeout> | null = null;

  const windowsResize = throttle(() => {
    width.value = window.innerWidth;
    const el = options.container?.value;
    if (!el) return;
    el.classList.add("resizing");
    if (resizeTimeout) {
      clearTimeout(resizeTimeout);
    }
    resizeTimeout = setTimeout(() => {
      resizeTimeout = null;
      el.classList.remove("resizing");
    }, 150);
  }, 100);

  onMounted(() => window.addEventListener("resize", windowsResize));
  onBeforeUnmount(() => {
    window.removeEventListener("resize", windowsResize);
    if (resizeTimeout) {
      clearTimeout(resizeTimeout);
      resizeTimeout = null;
    }
  });
  return { numColumns, itemStyles };
}
