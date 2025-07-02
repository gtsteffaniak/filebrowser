const name = window.FileBrowser.Name;
const disableExternal = window.FileBrowser.DisableExternal;
const externalLinks = window.FileBrowser.ExternalLinks;
const baseURL = window.FileBrowser.BaseURL;
const staticURL = window.FileBrowser.StaticURL;
const darkMode = window.FileBrowser.darkMode;
const recaptcha = false;
const recaptchaKey = "";
const signup = window.FileBrowser.Signup;
const version = window.FileBrowser.Version;
const commitSHA = window.FileBrowser.CommitSHA;
const logoURL = `${staticURL}/img/logo.png`;
const noAuth = window.FileBrowser.NoAuth;
const loginPage = window.FileBrowser.LoginPage;
const enableThumbs = window.FileBrowser.EnableThumbs;
const externalUrl = window.FileBrowser.ExternalUrl
const onlyOfficeUrl = window.FileBrowser.OnlyOfficeUrl
const serverHasMultipleSources = window.FileBrowser.SourceCount > 1;
const oidcAvailable = window.FileBrowser.OidcAvailable;
const passwordAvailable = window.FileBrowser.PasswordAvailable;
const mediaAvailable = window.FileBrowser.MediaAvailable;
const muPdfAvailable = window.FileBrowser.MuPdfAvailable;
const updateAvailable = window.FileBrowser.UpdateAvailable;
const disableNavButtons = window.FileBrowser.DisableNavButtons;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: 'settings.profileSettings', component: 'ProfileSettings' },
  { id: 'shares', label: 'settings.shareSettings', component: 'SharesSettings', permissions: { share: true } },
  { id: 'api', label: 'api.title', component: 'ApiKeys', permissions: { api: true }  },
  //{ id: 'global', label: 'Global', component: 'GlobalSettings', permissions: { admin: true } },
  { id: 'users', label: 'settings.userManagement', component: 'UserManagement' },
];

export {
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
