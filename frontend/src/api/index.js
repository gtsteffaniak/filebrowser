// Backend route organization:
// - /api/users/ -> users.js
// - /api/auth/ -> auth.js
// - /api/resources/ -> resources.js
// - /api/access/ -> access.js
// - /api/share/ (and /api/shares/) -> share.js
// - /api/settings/ -> settings.js
// - /api/tools/ -> tools.js
// - /api/office/ -> office.js
// - /api/wopi/ -> wopi.js
// - /api/media/ -> media.js
// - /public/api/* -> public functions in respective files (e.g., resourcesApi.fetchFilesPublic)

import * as accessApi from "./access.js";
import * as authApi from "./auth.js";
import * as mediaApi from "./media.js";
import * as officeApi from "./office.js";
import * as resourcesApi from "./resources.js";
import * as settingsApi from "./settings.js";
import * as shareApi from "./share.js";
import * as toolsApi from "./tools.js";
import * as usersApi from "./users.js";
import * as wopiApi from "./wopi.js";
import * as quotasApi from "./quotas.js";

export {
    accessApi,
    authApi,
    mediaApi,
    officeApi,
    quotasApi,
    resourcesApi,
    settingsApi,
    shareApi,
    toolsApi,
    usersApi,
    wopiApi,
};
