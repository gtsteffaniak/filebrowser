import i18n from "@/i18n/index.ts";
import { getIconClass } from "./material-symbols.js";

declare global {
  interface Window {
    // biome-ignore lint/suspicious/noExplicitAny: server-provided config, not yet typed
    globalVars?: any;
  }
}

const appConfigEl = document.querySelector('#app-config');
// biome-ignore lint/suspicious/noExplicitAny: server-provided config, not yet typed
const globalVars: any = appConfigEl?.textContent
  ? JSON.parse(appConfigEl.textContent)
  : window.globalVars;
if (typeof window !== 'undefined') {
  window.globalVars = globalVars;
}
const origin = window.location.origin;

export interface SettingsSection {
  id: string;
  /** Translated on every call, so the label follows the current locale */
  label?: () => string;
  component?: string;
  icon: string;
  permissions?: Record<string, boolean>;
}

export interface SettingsEntry extends SettingsSection {
  sections?: (SettingsSection & { label: () => string })[];
}

const settings: SettingsEntry[] = [
  {
    id: 'profile',
    component: 'ProfileSettings',
    icon: 'person',
    sections: [
      { id: 'listingOptions', label: () => i18n.global.t('settings.listingOptions'), icon: 'view_list' },
      { id: 'thumbnailOptions', label: () => i18n.global.t('profileSettings.thumbnailOptions'), icon: 'image' },
      { id: 'sidebarOptions', label: () => i18n.global.t('profileSettings.sidebarOptions'), icon: 'dock_to_right' },
      { id: 'searchOptions', label: () => i18n.global.t('settings.searchOptions'), icon: 'search' },
      { id: 'fileViewerOptions', label: () => i18n.global.t('profileSettings.fileViewerOptions'), icon: 'play_circle' },
      { id: 'themeLanguage', label: () => i18n.global.t('profileSettings.themeAndLanguage'), icon: 'palette' },
      { id: 'accountOptions', label: () => i18n.global.t('profileSettings.accountOptions'), icon: 'person' },
    ],
  },
  { id: 'fileLoading', label: () => i18n.global.t('fileLoading.title'), component: 'FileLoading', icon: 'cloud_download' },
  { id: 'notifications', label: () => i18n.global.t('notifications.title'), component: 'NotificationsSettings', icon: 'notifications' },
  { id: 'shares', component: 'SharesSettings', permissions: { share: true }, icon: 'share' },
  { id: 'api', label: () => i18n.global.t('api.title'), component: 'ApiKeys', permissions: { api: true }, icon: 'key' },
  {
    id: 'users',
    component: 'UserManagement',
    icon: 'group',
    sections: [
      { id: 'main', label: () => i18n.global.t('general.users', { suffix: '' }), permissions: { admin: true }, icon: 'person' },
      { id: 'access', label: () => i18n.global.t('general.access'), component: 'AccessSettings', permissions: { admin: true }, icon: 'lock' },
      { id: 'groups', label: () => i18n.global.t('general.groups'), component: 'GroupsSettings', permissions: { admin: true }, icon: 'groups' },
    ],
  },
  {
    id: 'media',
    label: () => i18n.global.t('general.media'),
    component: 'MediaSettings',
    permissions: { admin: true },
    icon: 'movie',
  },
  { id: 'systemAdmin', label: () => i18n.global.t('settings.systemAdmin'), component: 'SystemAdmin', permissions: { admin: true }, icon: 'admin_panel_settings' },
];

const previewViews = [
  'preview',
  'mediaPlayer',
  'markdownViewer',
  'epubViewer',
  'docViewer',
  'onlyOfficeEditor',
  'wopiEditor',
  'editor',
  'loading',
  'threeJsViewer'
];

export interface Tool {
  id: string;
  name: string;
  description: string;
  icon: string;
  path: string;
  component: string;
}

// Translated on every call, so the names follow the current locale
const getTools = (): Tool[] => [
  {
    id: "sizeViewer",
    name: i18n.global.t("tools.sizeAnalyzer.name"),
    description: i18n.global.t("tools.sizeAnalyzer.description"),
    icon: "analytics",
    path: "/tools/sizeViewer",
    component: "SizeViewer",
  },
  {
    id: "duplicateFinder",
    name: i18n.global.t("tools.duplicateFinder.name"),
    description: i18n.global.t("tools.duplicateFinder.description"),
    icon: "content_copy",
    path: "/tools/duplicateFinder",
    component: "DuplicateFinder",
  },
  {
    id: "advancedSearch",
    name: i18n.global.t("tools.advancedSearch.name"),
    description: i18n.global.t("tools.advancedSearch.description"),
    icon: "manage_search",
    path: "/tools/advancedSearch",
    component: "AdvancedSearch",
  },
  {
    id: "materialIconPicker",
    name: i18n.global.t("tools.materialIconPicker.name"),
    description: i18n.global.t("tools.materialIconPicker.description"),
    icon: "interests",
    path: "/tools/materialIconPicker",
    component: "MaterialIconPicker",
  },
  {
    id: "fileWatcher",
    name: i18n.global.t("tools.fileWatcher.name"),
    description: i18n.global.t("tools.fileWatcher.description"),
    icon: "visibility",
    path: "/tools/fileWatcher",
    component: "FileWatcher",
  },
  {
    id: "activityViewer",
    name: i18n.global.t("tools.activityViewer.name"),
    description: i18n.global.t("tools.activityViewer.description"),
    icon: "monitoring",
    path: "/tools/activityViewer",
    component: "ActivityViewer",
  },
];

// Resolve tools lazily each call so locale and new registrations stay accurate
const tools = () => getTools();

export {
  getIconClass, // Re-exported from material-symbols.js for convenience
  globalVars,
  origin,
  previewViews,
  settings,
  tools,
};
