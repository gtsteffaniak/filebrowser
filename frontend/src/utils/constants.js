const name = window.FileBrowser.Name;
const disableExternal = window.FileBrowser.DisableExternal;
const externalLinks = window.FileBrowser.ExternalLinks;
const disableUsedPercentage = window.FileBrowser.DisableUsedPercentage;
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
const resizePreview = window.FileBrowser.ResizePreview;
const externalUrl = window.FileBrowser.ExternalUrl
const onlyOfficeUrl = window.FileBrowser.OnlyOfficeUrl
const serverHasMultipleSources = window.FileBrowser.SourceCount > 1;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: 'Profile Management', component: 'ProfileSettings' },
  { id: 'shares', label: 'Share Management', component: 'SharesSettings', perm: { share: true } },
  { id: 'api', label: 'API Keys', component: 'ApiKeys', perm: { api: true }  },
  //{ id: 'global', label: 'Global', component: 'GlobalSettings', perm: { admin: true } },
  { id: 'users', label: 'User Management', component: 'UserManagement' },
]

export {
  serverHasMultipleSources,
  name,
  externalUrl,
  disableExternal,
  externalLinks,
  disableUsedPercentage,
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
  resizePreview,
  origin,
  darkMode,
  settings,
  onlyOfficeUrl,
};
