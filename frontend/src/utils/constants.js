const name = window.FileBrowser.Name || "File Browser";
const disableExternal = window.FileBrowser.DisableExternal;
const disableUsedPercentage = window.FileBrowser.DisableUsedPercentage;
const baseURL = window.FileBrowser.BaseURL;
const staticURL = window.FileBrowser.StaticURL;
const darkMode = window.FileBrowser.darkMode;
const recaptcha = window.FileBrowser.ReCaptcha;
const recaptchaKey = window.FileBrowser.ReCaptchaKey;
const signup = window.FileBrowser.Signup;
const version = window.FileBrowser.Version;
const commitSHA = window.FileBrowser.CommitSHA;
const logoURL = `${staticURL}/img/logo.png`;
const noAuth = window.FileBrowser.NoAuth;
const authMethod = window.FileBrowser.AuthMethod;
const loginPage = window.FileBrowser.LoginPage;
const enableThumbs = window.FileBrowser.EnableThumbs;
const resizePreview = window.FileBrowser.ResizePreview;
const enableExec = window.FileBrowser.EnableExec;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: 'Profile Management', component: 'ProfileSettings' },
  { id: 'shares', label: 'Share Management', component: 'SharesSettings' },
  { id: 'global', label: 'Global', component: 'GlobalSettings' },
  { id: 'users', label: 'User Management', component: 'UserManagement' },
]

export {
  name,
  disableExternal,
  disableUsedPercentage,
  baseURL,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
  version,
  commitSHA,
  noAuth,
  authMethod,
  loginPage,
  enableThumbs,
  resizePreview,
  enableExec,
  origin,
  darkMode,
  settings
};
