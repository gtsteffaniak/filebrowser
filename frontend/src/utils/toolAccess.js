import { getters, state } from "@/store";
import { tools } from "@/utils/constants";
import { getObjectProperty } from "@/utils/object";

export function toolIdFromPath(path) {
  const tool = tools().find((entry) => entry.path === path);
  return tool?.id || null;
}

function policyItemFor(toolId, policyItems) {
  if (!Array.isArray(policyItems)) {
    return null;
  }
  return policyItems.find((entry) => entry?.toolId === toolId) || null;
}

export function resolveHasToolAccess(toolId, {
  user = state.user,
  policyItems = state.toolAccessDefaultsPolicy?.items,
  isAdmin = getters.isAdmin(),
} = {}) {
  if (!toolId) {
    return false;
  }
  if (isAdmin) {
    return true;
  }
  const policyItem = policyItemFor(toolId, policyItems);
  if (policyItem?.enforced) {
    return !!policyItem.enabled;
  }
  const userValue = getObjectProperty(user?.toolAccess, toolId);
  if (typeof userValue === "boolean") {
    return userValue;
  }
  if (policyItem) {
    return !!policyItem.enabled;
  }
  return true;
}

export function hasToolAccess(toolId) {
  return resolveHasToolAccess(toolId);
}

export function isToolEnforced(toolId, policyItems = state.toolAccessDefaultsPolicy?.items) {
  const item = policyItemFor(toolId, policyItems);
  return !!item?.enforced;
}

export function availableTools() {
  return tools().filter((tool) => hasToolAccess(tool.id));
}

export function catalogToolById(toolId) {
  return tools().find((tool) => tool.id === toolId) || null;
}

export function hasAnyToolAccess() {
  return availableTools().length > 0;
}

export function toolAccessPolicyItems() {
  const items = state.toolAccessDefaultsPolicy?.items;
  return Array.isArray(items) ? items : [];
}

export function normalizeUserToolAccess(toolAccess = {}, policyItems = []) {
  const normalized = {};
  const items = Array.isArray(policyItems) ? policyItems : [];
  for (const tool of tools()) {
    const policyItem = policyItemFor(tool.id, items);
    if (policyItem?.enforced) {
      normalized[tool.id] = !!policyItem.enabled;
      continue;
    }
    const userValue = getObjectProperty(toolAccess, tool.id);
    if (typeof userValue === "boolean") {
      normalized[tool.id] = userValue;
      continue;
    }
    normalized[tool.id] = policyItem ? !!policyItem.enabled : true;
  }
  return normalized;
}
