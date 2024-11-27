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
  if (command === "serve") {
    return {
      plugins,
      resolve,
      server: {
        proxy: {
          "/api/command": {
            target: "ws://127.0.0.1:8080",
            ws: true
          },
          "/api": "http://127.0.0.1:8080"
        }
      }
    };
  } else {
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
      }
    };
  }
});
export {
  vite_config_default as default
};
//# sourceMappingURL=data:application/json;base64,ewogICJ2ZXJzaW9uIjogMywKICAic291cmNlcyI6IFsidml0ZS5jb25maWcudHMiXSwKICAic291cmNlc0NvbnRlbnQiOiBbImNvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9kaXJuYW1lID0gXCIvVXNlcnMvc3RlZmZhZy9naXQvcGVyc29uYWwvZmlsZWJyb3dzZXIvZnJvbnRlbmRcIjtjb25zdCBfX3ZpdGVfaW5qZWN0ZWRfb3JpZ2luYWxfZmlsZW5hbWUgPSBcIi9Vc2Vycy9zdGVmZmFnL2dpdC9wZXJzb25hbC9maWxlYnJvd3Nlci9mcm9udGVuZC92aXRlLmNvbmZpZy50c1wiO2NvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9pbXBvcnRfbWV0YV91cmwgPSBcImZpbGU6Ly8vVXNlcnMvc3RlZmZhZy9naXQvcGVyc29uYWwvZmlsZWJyb3dzZXIvZnJvbnRlbmQvdml0ZS5jb25maWcudHNcIjtpbXBvcnQgcGF0aCBmcm9tIFwibm9kZTpwYXRoXCI7XG5pbXBvcnQgeyBkZWZpbmVDb25maWcgfSBmcm9tIFwidml0ZVwiO1xuaW1wb3J0IHZ1ZSBmcm9tIFwiQHZpdGVqcy9wbHVnaW4tdnVlXCI7XG5pbXBvcnQgVnVlSTE4blBsdWdpbiBmcm9tIFwiQGludGxpZnkvdW5wbHVnaW4tdnVlLWkxOG4vdml0ZVwiO1xuaW1wb3J0IHsgY29tcHJlc3Npb24gfSBmcm9tIFwidml0ZS1wbHVnaW4tY29tcHJlc3Npb24yXCI7XG5cbmNvbnN0IHBsdWdpbnMgPSBbXG4gIHZ1ZSgpLFxuICBWdWVJMThuUGx1Z2luKHtcbiAgICBpbmNsdWRlOiBbcGF0aC5yZXNvbHZlKF9fZGlybmFtZSwgXCIuL3NyYy9pMThuLyoqLyouanNvblwiKV0sXG4gIH0pLFxuICBjb21wcmVzc2lvbih7XG4gICAgaW5jbHVkZTogL1xcLihqc3x3b2ZmMnx3b2ZmKShcXD8uKik/JC9pLFxuICAgIGRlbGV0ZU9yaWdpbmFsQXNzZXRzOiB0cnVlLFxuICB9KSxcbl07XG5cbmNvbnN0IHJlc29sdmUgPSB7XG4gIGFsaWFzOiB7XG4gICAgXCJAXCI6IHBhdGgucmVzb2x2ZShfX2Rpcm5hbWUsIFwic3JjXCIpLFxuICB9LFxufTtcblxuLy8gaHR0cHM6Ly92aXRlanMuZGV2L2NvbmZpZy9cbmV4cG9ydCBkZWZhdWx0IGRlZmluZUNvbmZpZygoeyBjb21tYW5kIH0pID0+IHtcbiAgaWYgKGNvbW1hbmQgPT09IFwic2VydmVcIikge1xuICAgIHJldHVybiB7XG4gICAgICBwbHVnaW5zLFxuICAgICAgcmVzb2x2ZSxcbiAgICAgIHNlcnZlcjoge1xuICAgICAgICBwcm94eToge1xuICAgICAgICAgIFwiL2FwaS9jb21tYW5kXCI6IHtcbiAgICAgICAgICAgIHRhcmdldDogXCJ3czovLzEyNy4wLjAuMTo4MDgwXCIsXG4gICAgICAgICAgICB3czogdHJ1ZSxcbiAgICAgICAgICB9LFxuICAgICAgICAgIFwiL2FwaVwiOiBcImh0dHA6Ly8xMjcuMC4wLjE6ODA4MFwiLFxuICAgICAgICB9LFxuICAgICAgfSxcbiAgICB9O1xuICB9IGVsc2Uge1xuICAgIC8vIGNvbW1hbmQgPT09ICdidWlsZCdcbiAgICByZXR1cm4ge1xuICAgICAgcGx1Z2lucyxcbiAgICAgIHJlc29sdmUsXG4gICAgICBiYXNlOiBcIlwiLFxuICAgICAgYnVpbGQ6IHtcbiAgICAgICAgcm9sbHVwT3B0aW9uczoge1xuICAgICAgICAgIGlucHV0OiB7XG4gICAgICAgICAgICBpbmRleDogcGF0aC5yZXNvbHZlKF9fZGlybmFtZSwgXCIuL3B1YmxpYy9pbmRleC5odG1sXCIpLFxuICAgICAgICAgIH0sXG4gICAgICAgICAgb3V0cHV0OiB7XG4gICAgICAgICAgICBtYW51YWxDaHVua3M6IChpZCkgPT4ge1xuICAgICAgICAgICAgICBpZiAoaWQuaW5jbHVkZXMoXCJpMThuL1wiKSkge1xuICAgICAgICAgICAgICAgIHJldHVybiBcImkxOG5cIjtcbiAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgfSxcbiAgICAgICAgICB9LFxuICAgICAgICB9LFxuICAgICAgfSxcbiAgICAgIGV4cGVyaW1lbnRhbDoge1xuICAgICAgICByZW5kZXJCdWlsdFVybChmaWxlbmFtZSwgeyBob3N0VHlwZSB9KSB7XG4gICAgICAgICAgaWYgKGhvc3RUeXBlID09PSBcImpzXCIpIHtcbiAgICAgICAgICAgIHJldHVybiB7IHJ1bnRpbWU6IGB3aW5kb3cuX19wcmVwZW5kU3RhdGljVXJsKFwiJHtmaWxlbmFtZX1cIilgIH07XG4gICAgICAgICAgfSBlbHNlIGlmIChob3N0VHlwZSA9PT0gXCJodG1sXCIpIHtcbiAgICAgICAgICAgIHJldHVybiBge3sgLlN0YXRpY1VSTCB9fS8ke2ZpbGVuYW1lfWA7XG4gICAgICAgICAgfSBlbHNlIHtcbiAgICAgICAgICAgIHJldHVybiB7IHJlbGF0aXZlOiB0cnVlIH07XG4gICAgICAgICAgfVxuICAgICAgICB9LFxuICAgICAgfSxcbiAgICB9O1xuICB9XG59KTtcbiJdLAogICJtYXBwaW5ncyI6ICI7QUFBa1UsT0FBTyxVQUFVO0FBQ25WLFNBQVMsb0JBQW9CO0FBQzdCLE9BQU8sU0FBUztBQUNoQixPQUFPLG1CQUFtQjtBQUMxQixTQUFTLG1CQUFtQjtBQUo1QixJQUFNLG1DQUFtQztBQU16QyxJQUFNLFVBQVU7QUFBQSxFQUNkLElBQUk7QUFBQSxFQUNKLGNBQWM7QUFBQSxJQUNaLFNBQVMsQ0FBQyxLQUFLLFFBQVEsa0NBQVcsc0JBQXNCLENBQUM7QUFBQSxFQUMzRCxDQUFDO0FBQUEsRUFDRCxZQUFZO0FBQUEsSUFDVixTQUFTO0FBQUEsSUFDVCxzQkFBc0I7QUFBQSxFQUN4QixDQUFDO0FBQ0g7QUFFQSxJQUFNLFVBQVU7QUFBQSxFQUNkLE9BQU87QUFBQSxJQUNMLEtBQUssS0FBSyxRQUFRLGtDQUFXLEtBQUs7QUFBQSxFQUNwQztBQUNGO0FBR0EsSUFBTyxzQkFBUSxhQUFhLENBQUMsRUFBRSxRQUFRLE1BQU07QUFDM0MsTUFBSSxZQUFZLFNBQVM7QUFDdkIsV0FBTztBQUFBLE1BQ0w7QUFBQSxNQUNBO0FBQUEsTUFDQSxRQUFRO0FBQUEsUUFDTixPQUFPO0FBQUEsVUFDTCxnQkFBZ0I7QUFBQSxZQUNkLFFBQVE7QUFBQSxZQUNSLElBQUk7QUFBQSxVQUNOO0FBQUEsVUFDQSxRQUFRO0FBQUEsUUFDVjtBQUFBLE1BQ0Y7QUFBQSxJQUNGO0FBQUEsRUFDRixPQUFPO0FBRUwsV0FBTztBQUFBLE1BQ0w7QUFBQSxNQUNBO0FBQUEsTUFDQSxNQUFNO0FBQUEsTUFDTixPQUFPO0FBQUEsUUFDTCxlQUFlO0FBQUEsVUFDYixPQUFPO0FBQUEsWUFDTCxPQUFPLEtBQUssUUFBUSxrQ0FBVyxxQkFBcUI7QUFBQSxVQUN0RDtBQUFBLFVBQ0EsUUFBUTtBQUFBLFlBQ04sY0FBYyxDQUFDLE9BQU87QUFDcEIsa0JBQUksR0FBRyxTQUFTLE9BQU8sR0FBRztBQUN4Qix1QkFBTztBQUFBLGNBQ1Q7QUFBQSxZQUNGO0FBQUEsVUFDRjtBQUFBLFFBQ0Y7QUFBQSxNQUNGO0FBQUEsTUFDQSxjQUFjO0FBQUEsUUFDWixlQUFlLFVBQVUsRUFBRSxTQUFTLEdBQUc7QUFDckMsY0FBSSxhQUFhLE1BQU07QUFDckIsbUJBQU8sRUFBRSxTQUFTLDhCQUE4QixRQUFRLEtBQUs7QUFBQSxVQUMvRCxXQUFXLGFBQWEsUUFBUTtBQUM5QixtQkFBTyxvQkFBb0IsUUFBUTtBQUFBLFVBQ3JDLE9BQU87QUFDTCxtQkFBTyxFQUFFLFVBQVUsS0FBSztBQUFBLFVBQzFCO0FBQUEsUUFDRjtBQUFBLE1BQ0Y7QUFBQSxJQUNGO0FBQUEsRUFDRjtBQUNGLENBQUM7IiwKICAibmFtZXMiOiBbXQp9Cg==
