import i18n from '@/i18n';

const globalVars = window.globalVars;
const logoURL = `${globalVars.staticURL}/img/logo.png`;
const serverHasMultipleSources = globalVars.sourceCount > 1;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: 'settings.profileSettings', component: 'ProfileSettings' },
  { id: 'fileLoading', label: 'fileLoading.title', component: 'FileLoading' },
  { id: 'shares', label: 'settings.shareSettings', component: 'SharesSettings', permissions: { share: true } },
  { id: 'api', label: 'api.title', component: 'ApiKeys', permissions: { api: true }  },
  { id: 'users', label: 'settings.userManagement', component: 'UserManagement' },
  { id: 'access', label: 'access.accessManagement', component: 'AccessSettings', permissions: { admin: true } },
  { id: 'systemAdmin', label: 'settings.systemAdmin', component: 'SystemAdmin', permissions: { admin: true } },
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
  },
  {
    name: i18n.global.t("tools.duplicateFinder.name"),
    description: i18n.global.t("tools.duplicateFinder.description"),
    icon: "content_copy",
    path: "/tools/duplicateFinder",
  },
];

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
  serverHasMultipleSources,
  logoURL,
  origin,
  settings,
  previewViews,
  tools,
};
