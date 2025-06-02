import i18n from '@/i18n'; // Import the default export (your i18n instance)

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
const pdfAvailable = window.FileBrowser.PdfAvailable;
const origin = window.location.origin;

const settings = [
  { id: 'profile', label: i18n.global.t('settings.profileSettings'), component: 'ProfileSettings' },
  { id: 'shares', label: i18n.global.t('settings.shareSettings'), component: 'SharesSettings', permissions: { share: true } },
  { id: 'api', label: i18n.global.t('api.title'), component: 'ApiKeys', permissions: { api: true }  },
  //{ id: 'global', label: 'Global', component: 'GlobalSettings', permissions: { admin: true } },
  { id: 'users', label: i18n.global.t('settings.userManagement'), component: 'UserManagement' },
];

export {
  pdfAvailable,
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
