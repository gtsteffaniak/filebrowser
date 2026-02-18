import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import { compression } from "vite-plugin-compression2";
import checker from "vite-plugin-checker";

const isDevBuild = process.env.DEV_BUILD === "true";

const plugins = [
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__dirname, "./src/i18n/**/*.json")],
  }),
  // Only compress in production builds
  !isDevBuild && compression({
    include: /\.(js|woff2|woff)(\?.*)?$/i,
    deleteOriginalAssets: true,
  }),
  // Disable checker in watch mode to prevent task failures
  !isDevBuild && checker({
    typescript: false, // Disable redundant check
    vueTsc: {
      tsconfigPath: "./tsconfig.json",
    },
  }),
].filter(Boolean);

const resolve = {
  alias: {
    "@": path.resolve(__dirname, "src"),
  },
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  return {
    plugins,
    resolve,
    base: "",
    build: {
      // Optimize for watch mode stability
      watch: isDevBuild ? {
        // Add buildDelay to batch multiple changes
        buildDelay: 500,
      } : null,
      chunkSizeWarningLimit: 5000,
      rollupOptions: {
        input: {
          index: path.resolve(__dirname, "./public/index.html"),
        },
        output: {
          manualChunks: (id) => {
            if (id.includes("i18n/")) {
              return "i18n";
            }
          },
        },
        // Better error handling in watch mode
        onwarn(warning, warn) {
          // Suppress certain warnings in dev mode
          if (isDevBuild && warning.code === 'UNUSED_EXTERNAL_IMPORT') {
            return;
          }
          warn(warning);
        },
      },
    },
    experimental: {
      renderBuiltUrl(filename, { hostType }) {
        if (hostType === "js") {
          // Use relative paths instead of runtime function
          return { relative: true };
        } else if (hostType === "html") {
          return `{{ .htmlVars.staticURL }}/${filename}`;
        } else {
          return { relative: true };
        }
      },
    },
    test: {
      globals: true,
      include: ["src/**/*.test.js"],
      exclude: ["src/**/*.vue"],
      environment: "jsdom",
      setupFiles: "tests/mocks/setup.js",
    },
  };
});
