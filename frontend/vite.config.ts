import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
// import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import { compression } from "vite-plugin-compression2";
import checker from "vite-plugin-checker";

// Check if this is a development build (build:dev or watch) vs production build (build)
// Use a custom environment variable to detect development builds
const isDevBuild = process.env.DEV_BUILD === 'true' || 
                   process.env.npm_lifecycle_event === 'build:dev' || 
                   process.env.npm_lifecycle_event === 'watch';

// Debug: Log build configuration
console.log('Build configuration:', {
  DEV_BUILD: process.env.DEV_BUILD,
  npm_lifecycle_event: process.env.npm_lifecycle_event,
  isDevBuild: isDevBuild
});

// Note: We removed the Vite i18n plugin and are handling conditional loading manually
// in the i18n configuration file using import.meta.env.DEV_BUILD

const plugins = [
  vue(),
  // VueI18nPlugin removed - handling i18n loading manually
  compression({
    include: /\.(js|woff2|woff)(\?.*)?$/i,
    deleteOriginalAssets: true,
  }),
  checker({
    typescript: {
      buildMode: true,
    },
    vueTsc: {
      tsconfigPath: "./tsconfig.json",
    },
    eslint: {
      lintCommand: 'eslint "./src/**/*.{js,vue,ts}"',
    },
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
    define: {
      // Pass the DEV_BUILD flag to the client code
      'import.meta.env.DEV_BUILD': JSON.stringify(process.env.DEV_BUILD || 'false'),
    },
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
