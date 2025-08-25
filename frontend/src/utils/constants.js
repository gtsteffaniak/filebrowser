const name = window.filebrowser.name;
const disableExternal = window.filebrowser.disableExternal;
const externalLinks = window.filebrowser.externalLinks;
const baseURL = window.filebrowser.baseURL;
const staticURL = window.filebrowser.staticURL;
const darkMode = window.filebrowser.darkMode;
const recaptcha = window.filebrowser.recaptcha;
const recaptchaKey = window.filebrowser.recaptchaKey;
const signup = window.filebrowser.signup;
const version = window.filebrowser.version;
const commitSHA = window.filebrowser.commitSHA;
const logoURL = `${staticURL}/img/logo.png`;
const noAuth = window.filebrowser.noAuth;
const loginPage = window.filebrowser.loginPage;
const enableThumbs = window.filebrowser.enableThumbs;
const externalUrl = window.filebrowser.externalUrl
const onlyOfficeUrl = window.filebrowser.onlyOfficeUrl
const serverHasMultipleSources = window.filebrowser.sourceCount > 1;
const oidcAvailable = window.filebrowser.oidcAvailable;
const passwordAvailable = window.filebrowser.passwordAvailable;
const mediaAvailable = window.filebrowser.mediaAvailable;
const muPdfAvailable = window.filebrowser.muPdfAvailable;
const updateAvailable = window.filebrowser.updateAvailable;
const disableNavButtons = window.filebrowser.disableNavButtons;
const userSelectableThemes = window.filebrowser.userSelectableThemes;
const shareInfo = window.filebrowser.share;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: 'settings.profileSettings', component: 'ProfileSettings' },
  { id: 'fileLoading', label: 'fileLoading.title', component: 'FileLoading' },
  { id: 'shares', label: 'settings.shareSettings', component: 'SharesSettings', permissions: { share: true } },
  { id: 'api', label: 'api.title', component: 'ApiKeys', permissions: { api: true }  },
  //{ id: 'global', label: 'Global', component: 'GlobalSettings', permissions: { admin: true } },
  { id: 'users', label: 'settings.userManagement', component: 'UserManagement' },
  { id: 'access', label: 'access.accessManagement', component: 'AccessSettings', permissions: { admin: true } },
];

export {
  shareInfo,
  userSelectableThemes,
  disableNavButtons,
  updateAvailable,
  muPdfAvailable,
  mediaAvailable,
  oidcAvailable,
  passwordAvailable,
  serverHasMultipleSources,
  name,
  externalUrl,
  disableExternal,
  externalLinks,
  baseURL,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
  version,
  commitSHA,
  noAuth,
  loginPage,
  enableThumbs,
  origin,
  darkMode,
  settings,
  onlyOfficeUrl,
};
