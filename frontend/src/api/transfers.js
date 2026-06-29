import { fetchURL } from "./utils";
import { getApiPath } from "@/utils/url.js";

export async function listTransfers() {
  const apiPath = getApiPath("transfers");
  const res = await fetchURL(apiPath);
  return res.json();
}

export async function getTransfer(id) {
  const apiPath = getApiPath(`transfers/${id}`);
  const res = await fetchURL(apiPath);
  return res.json();
}

export async function cancelTransfer(id) {
  const apiPath = getApiPath(`transfers/${id}`);
  const res = await fetchURL(apiPath, { method: "DELETE" });
  return res.json();
}
