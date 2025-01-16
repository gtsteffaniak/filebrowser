// vite.config.ts
import path from "node:path";
import { defineConfig } from "file:///Users/steffag/git/personal/filebrowser/frontend/node_modules/vite/dist/node/index.js";
import vue from "file:///Users/steffag/git/personal/filebrowser/frontend/node_modules/@vitejs/plugin-vue/dist/index.mjs";
import VueI18nPlugin from "file:///Users/steffag/git/personal/filebrowser/frontend/node_modules/@intlify/unplugin-vue-i18n/lib/vite.mjs";
import { compression } from "file:///Users/steffag/git/personal/filebrowser/frontend/node_modules/vite-plugin-compression2/dist/index.mjs";
var __vite_injected_original_dirname = "/Users/steffag/git/personal/filebrowser/frontend";
var plugins = [
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__vite_injected_original_dirname, "./src/i18n/**/*.json")]
  }),
  compression({
    include: /\.(js|woff2|woff)(\?.*)?$/i,
    deleteOriginalAssets: true
  })
];
var resolve = {
  alias: {
    "@": path.resolve(__vite_injected_original_dirname, "src")
  }
};
var vite_config_default = defineConfig(({ command }) => {
  return {
    plugins,
    resolve,
    base: "",
    build: {
      rollupOptions: {
        input: {
          index: path.resolve(__vite_injected_original_dirname, "./public/index.html")
        },
        output: {
          manualChunks: (id) => {
            if (id.includes("i18n/")) {
              return "i18n";
            }
          }
        }
      }
    },
    experimental: {
      renderBuiltUrl(filename, { hostType }) {
        if (hostType === "js") {
          return { runtime: `window.__prependStaticUrl("${filename}")` };
        } else if (hostType === "html") {
          return `{{ .StaticURL }}/${filename}`;
        } else {
          return { relative: true };
        }
      }
    },
    test: {
      globals: true,
      include: ["src/**/*.test.js"],
      exclude: ["src/**/*.vue"],
      environment: "jsdom",
      setupFiles: "tests/mocks/setup.js"
    }
  };
});
export {
  vite_config_default as default
};
//# sourceMappingURL=data:application/json;base64,ewogICJ2ZXJzaW9uIjogMywKICAic291cmNlcyI6IFsidml0ZS5jb25maWcudHMiXSwKICAic291cmNlc0NvbnRlbnQiOiBbImNvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9kaXJuYW1lID0gXCIvVXNlcnMvc3RlZmZhZy9naXQvcGVyc29uYWwvZmlsZWJyb3dzZXIvZnJvbnRlbmRcIjtjb25zdCBfX3ZpdGVfaW5qZWN0ZWRfb3JpZ2luYWxfZmlsZW5hbWUgPSBcIi9Vc2Vycy9zdGVmZmFnL2dpdC9wZXJzb25hbC9maWxlYnJvd3Nlci9mcm9udGVuZC92aXRlLmNvbmZpZy50c1wiO2NvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9pbXBvcnRfbWV0YV91cmwgPSBcImZpbGU6Ly8vVXNlcnMvc3RlZmZhZy9naXQvcGVyc29uYWwvZmlsZWJyb3dzZXIvZnJvbnRlbmQvdml0ZS5jb25maWcudHNcIjtpbXBvcnQgcGF0aCBmcm9tIFwibm9kZTpwYXRoXCI7XG5pbXBvcnQgeyBkZWZpbmVDb25maWcgfSBmcm9tIFwidml0ZVwiO1xuaW1wb3J0IHZ1ZSBmcm9tIFwiQHZpdGVqcy9wbHVnaW4tdnVlXCI7XG5pbXBvcnQgVnVlSTE4blBsdWdpbiBmcm9tIFwiQGludGxpZnkvdW5wbHVnaW4tdnVlLWkxOG4vdml0ZVwiO1xuaW1wb3J0IHsgY29tcHJlc3Npb24gfSBmcm9tIFwidml0ZS1wbHVnaW4tY29tcHJlc3Npb24yXCI7XG5cbmNvbnN0IHBsdWdpbnMgPSBbXG4gIHZ1ZSgpLFxuICBWdWVJMThuUGx1Z2luKHtcbiAgICBpbmNsdWRlOiBbcGF0aC5yZXNvbHZlKF9fZGlybmFtZSwgXCIuL3NyYy9pMThuLyoqLyouanNvblwiKV0sXG4gIH0pLFxuICBjb21wcmVzc2lvbih7XG4gICAgaW5jbHVkZTogL1xcLihqc3x3b2ZmMnx3b2ZmKShcXD8uKik/JC9pLFxuICAgIGRlbGV0ZU9yaWdpbmFsQXNzZXRzOiB0cnVlLFxuICB9KSxcbl07XG5cbmNvbnN0IHJlc29sdmUgPSB7XG4gIGFsaWFzOiB7XG4gICAgXCJAXCI6IHBhdGgucmVzb2x2ZShfX2Rpcm5hbWUsIFwic3JjXCIpLFxuICB9LFxufTtcblxuLy8gaHR0cHM6Ly92aXRlanMuZGV2L2NvbmZpZy9cbmV4cG9ydCBkZWZhdWx0IGRlZmluZUNvbmZpZygoeyBjb21tYW5kIH0pID0+IHtcbiAgcmV0dXJuIHtcbiAgICBwbHVnaW5zLFxuICAgIHJlc29sdmUsXG4gICAgYmFzZTogXCJcIixcbiAgICBidWlsZDoge1xuICAgICAgcm9sbHVwT3B0aW9uczoge1xuICAgICAgICBpbnB1dDoge1xuICAgICAgICAgIGluZGV4OiBwYXRoLnJlc29sdmUoX19kaXJuYW1lLCBcIi4vcHVibGljL2luZGV4Lmh0bWxcIiksXG4gICAgICAgIH0sXG4gICAgICAgIG91dHB1dDoge1xuICAgICAgICAgIG1hbnVhbENodW5rczogKGlkKSA9PiB7XG4gICAgICAgICAgICBpZiAoaWQuaW5jbHVkZXMoXCJpMThuL1wiKSkge1xuICAgICAgICAgICAgICByZXR1cm4gXCJpMThuXCI7XG4gICAgICAgICAgICB9XG4gICAgICAgICAgfSxcbiAgICAgICAgfSxcbiAgICAgIH0sXG4gICAgfSxcbiAgICBleHBlcmltZW50YWw6IHtcbiAgICAgIHJlbmRlckJ1aWx0VXJsKGZpbGVuYW1lLCB7IGhvc3RUeXBlIH0pIHtcbiAgICAgICAgaWYgKGhvc3RUeXBlID09PSBcImpzXCIpIHtcbiAgICAgICAgICByZXR1cm4geyBydW50aW1lOiBgd2luZG93Ll9fcHJlcGVuZFN0YXRpY1VybChcIiR7ZmlsZW5hbWV9XCIpYCB9O1xuICAgICAgICB9IGVsc2UgaWYgKGhvc3RUeXBlID09PSBcImh0bWxcIikge1xuICAgICAgICAgIHJldHVybiBge3sgLlN0YXRpY1VSTCB9fS8ke2ZpbGVuYW1lfWA7XG4gICAgICAgIH0gZWxzZSB7XG4gICAgICAgICAgcmV0dXJuIHsgcmVsYXRpdmU6IHRydWUgfTtcbiAgICAgICAgfVxuICAgICAgfSxcbiAgICB9LFxuICAgIHRlc3Q6IHtcbiAgICAgIGdsb2JhbHM6IHRydWUsXG4gICAgICBpbmNsdWRlOiBbXCJzcmMvKiovKi50ZXN0LmpzXCJdLFxuICAgICAgZXhjbHVkZTogW1wic3JjLyoqLyoudnVlXCJdLFxuICAgICAgZW52aXJvbm1lbnQ6IFwianNkb21cIixcbiAgICAgIHNldHVwRmlsZXM6IFwidGVzdHMvbW9ja3Mvc2V0dXAuanNcIixcbiAgICB9LFxuICB9O1xufSk7XG4iXSwKICAibWFwcGluZ3MiOiAiO0FBQWtVLE9BQU8sVUFBVTtBQUNuVixTQUFTLG9CQUFvQjtBQUM3QixPQUFPLFNBQVM7QUFDaEIsT0FBTyxtQkFBbUI7QUFDMUIsU0FBUyxtQkFBbUI7QUFKNUIsSUFBTSxtQ0FBbUM7QUFNekMsSUFBTSxVQUFVO0FBQUEsRUFDZCxJQUFJO0FBQUEsRUFDSixjQUFjO0FBQUEsSUFDWixTQUFTLENBQUMsS0FBSyxRQUFRLGtDQUFXLHNCQUFzQixDQUFDO0FBQUEsRUFDM0QsQ0FBQztBQUFBLEVBQ0QsWUFBWTtBQUFBLElBQ1YsU0FBUztBQUFBLElBQ1Qsc0JBQXNCO0FBQUEsRUFDeEIsQ0FBQztBQUNIO0FBRUEsSUFBTSxVQUFVO0FBQUEsRUFDZCxPQUFPO0FBQUEsSUFDTCxLQUFLLEtBQUssUUFBUSxrQ0FBVyxLQUFLO0FBQUEsRUFDcEM7QUFDRjtBQUdBLElBQU8sc0JBQVEsYUFBYSxDQUFDLEVBQUUsUUFBUSxNQUFNO0FBQzNDLFNBQU87QUFBQSxJQUNMO0FBQUEsSUFDQTtBQUFBLElBQ0EsTUFBTTtBQUFBLElBQ04sT0FBTztBQUFBLE1BQ0wsZUFBZTtBQUFBLFFBQ2IsT0FBTztBQUFBLFVBQ0wsT0FBTyxLQUFLLFFBQVEsa0NBQVcscUJBQXFCO0FBQUEsUUFDdEQ7QUFBQSxRQUNBLFFBQVE7QUFBQSxVQUNOLGNBQWMsQ0FBQyxPQUFPO0FBQ3BCLGdCQUFJLEdBQUcsU0FBUyxPQUFPLEdBQUc7QUFDeEIscUJBQU87QUFBQSxZQUNUO0FBQUEsVUFDRjtBQUFBLFFBQ0Y7QUFBQSxNQUNGO0FBQUEsSUFDRjtBQUFBLElBQ0EsY0FBYztBQUFBLE1BQ1osZUFBZSxVQUFVLEVBQUUsU0FBUyxHQUFHO0FBQ3JDLFlBQUksYUFBYSxNQUFNO0FBQ3JCLGlCQUFPLEVBQUUsU0FBUyw4QkFBOEIsUUFBUSxLQUFLO0FBQUEsUUFDL0QsV0FBVyxhQUFhLFFBQVE7QUFDOUIsaUJBQU8sb0JBQW9CLFFBQVE7QUFBQSxRQUNyQyxPQUFPO0FBQ0wsaUJBQU8sRUFBRSxVQUFVLEtBQUs7QUFBQSxRQUMxQjtBQUFBLE1BQ0Y7QUFBQSxJQUNGO0FBQUEsSUFDQSxNQUFNO0FBQUEsTUFDSixTQUFTO0FBQUEsTUFDVCxTQUFTLENBQUMsa0JBQWtCO0FBQUEsTUFDNUIsU0FBUyxDQUFDLGNBQWM7QUFBQSxNQUN4QixhQUFhO0FBQUEsTUFDYixZQUFZO0FBQUEsSUFDZDtBQUFBLEVBQ0Y7QUFDRixDQUFDOyIsCiAgIm5hbWVzIjogW10KfQo=
