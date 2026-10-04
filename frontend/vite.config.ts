import path from "node:path";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import vue from "@vitejs/plugin-vue";
import { defineConfig, type Rolldown } from "vite";
import checker from "vite-plugin-checker";
import { compression } from "vite-plugin-compression2";
import stripLegacyCSS from "./scripts/postcss-strip-legacy.ts";
import protectTemplateStyles from "./scripts/vite-protect-template-styles.ts";

const isDevBuild = process.env.DEV_BUILD === "true";
const backendWebDist = path.resolve(import.meta.dirname, "../backend/internal/web/dist");

const resolve = {
  alias: {
    "@": path.resolve(import.meta.dirname, "src"),
  },
};

const css = {
  postcss: {
    plugins: [stripLegacyCSS()],
  },
};

const define = {
  __VUE_I18N_LEGACY_API__: JSON.stringify(false),
  __VUE_I18N_FULL_INSTALL__: JSON.stringify(false),
};

const test = {
  globals: true,
  include: [
    "src/**/*.test.{js,ts}",
    "tests/playwright/performance/**/*.test.ts",
  ],
  exclude: ["src/**/*.vue"],
  environment: "jsdom",
  setupFiles: "tests/mocks/setup.js",
};

const chunks: Record<string, string[]> = {
  store: ["/src/store/"],
  highlightjs: ["node_modules/highlight.js"],
  mammoth: ["node_modules/mammoth"],
  epubjs: ["node_modules/jszip", "node_modules/epubjs"],
  katex: ["node_modules/katex", "node_modules/marked-katex-extension"],
};

// Better error handling in watch mode: suppress certain warnings in dev builds.
const onwarn: NonNullable<Rolldown.InputOptions["onwarn"]> = (warning, warn) => {
  if (isDevBuild && warning.code === "UNUSED_EXTERNAL_IMPORT") {
    return;
  }
  warn(warning);
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  const isServe = command === "serve";

  const plugins = [
    ...protectTemplateStyles(),
    vue(),
    VueI18nPlugin({
      runtimeOnly: false,
      include: [path.resolve(import.meta.dirname, "./src/i18n/**/*.json")],
    }),
    // Only compress in production builds
    !isDevBuild && !isServe && compression({
      algorithms: ["gzip"],
      include: /\.(js|woff2|woff)(\?|$)/i,
      deleteOriginalAssets: true,
    }),
    // Disable checker in watch mode to prevent task failures
    !isDevBuild && !isServe && checker({
      typescript: false, // Disable redundant check
      vueTsc: {
        tsconfigPath: "./tsconfig.json",
      },
    }),
  ].filter(Boolean);

  if (isServe) {
    const devOrigin = process.env.VITE_DEV_ORIGIN || "http://localhost:8080";
    const devPort = Number.parseInt(process.env.VITE_DEV_PORT || "5173", 10);

    return {
      plugins,
      resolve,
      css,
      base: "/__vite/",
      publicDir: path.resolve(import.meta.dirname, "public"),
      server: {
        host: "127.0.0.1",
        port: devPort,
        strictPort: true,
        origin: devOrigin,
        // base already is /__vite/; do not set hmr.path or it becomes /__vite/__vite/
        hmr: {
          clientPort: Number.parseInt(process.env.VITE_DEV_CLIENT_PORT || "8080", 10),
        },
      },
      define,
      test,
    };
  }

  return {
    plugins,
    resolve,
    css,
    base: "",
    define,
    build: {
      outDir: backendWebDist,
      emptyOutDir: true,
      // Optimize for watch mode stability
      watch: isDevBuild ? {
        // Add buildDelay to batch multiple changes
        buildDelay: 500,
      } : null,
      target: "es2024",
      sourcemap: false,
      chunkSizeWarningLimit: 5000,
      rolldownOptions: {
        // vue-tsc and compression run after the bundle is built
        checks: { pluginTimings: false },
        input: {
          index: path.resolve(import.meta.dirname, "./public/index.html"),
        },
        output: {
          strictExecutionOrder: true,
          codeSplitting: {
            groups: [
              {
                debugName: "manual-chunks",
                name(id: string) {
                  for (const [name, needles] of Object.entries(chunks)) {
                    if (needles.some((n) => id.includes(n))) {
                      return name;
                    }
                  }
                  return null;
                },
              },
            ],
          },
        },
        onwarn,
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
    test,
  };
});
