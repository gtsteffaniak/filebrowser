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

import * as authApi from "./auth";
import * as usersApi from "./users";
import * as resourcesApi from "./resources";
import * as accessApi from "./access";
import * as shareApi from "./share";
import * as settingsApi from "./settings";
import * as toolsApi from "./tools";
import * as officeApi from "./office";
import * as mediaApi from "./media";

export { 
    authApi,
    usersApi,
    resourcesApi,
    accessApi,
    shareApi,
    settingsApi,
    toolsApi,
    officeApi,
    mediaApi
};

