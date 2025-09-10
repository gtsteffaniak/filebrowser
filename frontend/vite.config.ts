import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import { compression } from "vite-plugin-compression2";

const plugins = [
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__dirname, "./src/i18n/**/*.json")],
  }),
  compression({
    include: /\.(js|woff2|woff)(\?.*)?$/i,
    deleteOriginalAssets: true,
  }),
];

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
      },
    },
    experimental: {
      renderBuiltUrl(filename, { hostType }) {
        if (hostType === "js") {
          return { runtime: `window.__prependStaticUrl("${filename}")` };
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
