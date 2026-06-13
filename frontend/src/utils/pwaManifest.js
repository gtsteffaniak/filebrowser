import { globalVars } from "@/utils/constants";
import { getters, state } from "@/store";

function manifestBaseUrl() {
  const base = globalVars.baseURL || "/";
  return `${base}public/static/site.webmanifest`;
}

export function updateManifestLink() {
  const link = document.querySelector('link[rel="manifest"]');
  if (!link) {
    return;
  }

  if (getters.isShare() && state.shareInfo?.hash) {
    const base = globalVars.baseURL || "/";
    const start = `${base}public/share/${state.shareInfo.hash}/`;
    const params = new URLSearchParams({ start });
    if (state.shareInfo.title) {
      params.set("name", state.shareInfo.title);
    }
    if (state.shareInfo.description) {
      params.set("description", state.shareInfo.description);
    }
    link.href = `${manifestBaseUrl()}?${params.toString()}`;
    return;
  }

  link.href = manifestBaseUrl();
}
