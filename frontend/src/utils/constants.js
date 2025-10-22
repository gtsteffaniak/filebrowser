const globalVars = window.globalVars;
const logoURL = `${globalVars.staticURL}/img/logo.png`;
const serverHasMultipleSources = globalVars.sourceCount > 1;
const shareInfo = window.shareVars;
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

export {
  globalVars,
  shareInfo,
  serverHasMultipleSources,
  logoURL,
  origin,
  settings,
  previewViews,
};
