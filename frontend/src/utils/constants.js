import i18n from '@/i18n';
import { getIconClass } from './material-icons';

const globalVars = window.globalVars;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: 'settings.profileSettings', component: 'ProfileSettings', icon: 'person' },
  { id: 'fileLoading', label: 'fileLoading.title', component: 'FileLoading', icon: 'cloud_download' },
  { id: 'notifications', label: 'notifications.title', component: 'NotificationsSettings', icon: 'notifications' },
  { id: 'shares', label: 'settings.shareSettings', component: 'SharesSettings', permissions: { share: true }, icon: 'share' },
  { id: 'api', label: 'api.title', component: 'ApiKeys', permissions: { api: true }, icon: 'key' },
  { id: 'users', label: 'settings.userManagement', component: 'UserManagement', icon: 'group' },
  { id: 'access', label: 'access.accessManagement', component: 'AccessSettings', permissions: { admin: true }, icon: 'lock' },
  { id: 'systemAdmin', label: 'settings.systemAdmin', component: 'SystemAdmin', permissions: { admin: true }, icon: 'admin_panel_settings' },
];

const previewViews = [
  'preview',
  'markdownViewer',
  'epubViewer',
  'docViewer',
  'onlyOfficeEditor',
  'editor',
  'loading'
];

// Function that returns tools array with i18n validation
// This ensures the linter checks the translation keys
const getTools = () => [
  {
    name: i18n.global.t("tools.sizeAnalyzer.name"),
    description: i18n.global.t("tools.sizeAnalyzer.description"),
    icon: "analytics",
    path: "/tools/sizeViewer",
    component: "SizeViewer",
  },
  {
    name: i18n.global.t("tools.duplicateFinder.name"),
    description: i18n.global.t("tools.duplicateFinder.description"),
    icon: "content_copy",
    path: "/tools/duplicateFinder",
    component: "DuplicateFinder",
  },
  {
    name: i18n.global.t("tools.materialIconPicker.name"),
    description: i18n.global.t("tools.materialIconPicker.description"),
    icon: "interests",
    path: "/tools/materialIconPicker",
    component: "MaterialIconPicker",
  },
];

// Export tools as both a function and direct array for convenience
// Cache the tools array
let toolsCache = null;
const tools = () => {
  if (!toolsCache) {
    toolsCache = getTools();
  }
  return toolsCache;
};

export {
  globalVars,
  origin,
  settings,
  previewViews,
  tools,
  getIconClass, // Re-exported from material-icons.js for convenience
};
