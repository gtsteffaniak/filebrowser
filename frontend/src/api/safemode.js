import { fetchURL } from "./utils";
import { getApiPath } from "@/utils/url.js";

export async function getSafeModeItems() {
  const res = await fetchURL(getApiPath("api/safemode"));
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function addToSafeMode(items, pin) {
  const res = await fetchURL(getApiPath("api/safemode"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ items, pin }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.message || res.statusText);
  }
  return res.json();
}

export async function removeFromSafeMode(items, pin) {
  const res = await fetchURL(getApiPath("api/safemode"), {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ items, pin }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.message || res.statusText);
  }
  return res.json();
}

export async function verifySafeModePin(pin) {
  const res = await fetchURL(getApiPath("api/safemode/verify"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pin }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json(); // { valid: true/false }
}
