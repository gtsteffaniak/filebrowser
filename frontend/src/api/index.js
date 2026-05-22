// Backend route organization:
// - /api/users/ -> users.js
// - /api/auth/ -> auth.js
// - /api/resources/ -> resources.js
// - /api/access/ -> access.js
// - /api/share/ (and /api/shares/) -> share.js
// - /api/settings/ -> settings.js
// - /api/tools/ -> tools.js
// - /api/office/ -> office.js
// - /api/media/ -> media.js
// - /public/api/* -> public functions in respective files (e.g., resourcesApi.fetchFilesPublic)

import * as accessApi from "./access";
import * as authApi from "./auth";
import * as mediaApi from "./media";
import * as officeApi from "./office";
import * as resourcesApi from "./resources";
import * as settingsApi from "./settings";
import * as shareApi from "./share";
import * as toolsApi from "./tools";
import * as usersApi from "./users";

export {
    accessApi,
    authApi,
    mediaApi,
    officeApi,
    resourcesApi,
    settingsApi,
    shareApi,
    toolsApi,
    usersApi,
};
