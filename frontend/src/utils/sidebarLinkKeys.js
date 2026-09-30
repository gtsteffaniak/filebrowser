function normalizeTarget(target) {
  const t = (target || "").trim();
  if (!t || t === "#") {
    return "/";
  }
  if (!t.startsWith("/")) {
    return `/${t}`;
  }
  if (t !== "/" && t.endsWith("/")) {
    return t.slice(0, -1);
  }
  return t;
}

function isSourceCategory(category) {
  return (
    category === "source" ||
    category === "source-minimal" ||
    category === "source-alt" ||
    category === "source-hybrid" ||
    category === "source-hybrid-2"
  );
}

/** Stable sidebar link key (matches backend LinkKeyForDisplay). */
export function sidebarLinkKey(link) {
  if (!link) {
    return "";
  }
  if (isSourceCategory(link.category) && link.sourceName) {
    return `source:${link.sourceName}:${normalizeTarget(link.target)}`;
  }
  if (link.category === "tool" && link.target === "/tools") {
    return "tool:/tools";
  }
  return `${link.category}:${(link.target || "").trim()}`;
}
