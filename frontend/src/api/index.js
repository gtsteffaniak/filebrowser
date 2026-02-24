// Backend route organization:
// - /api/users/ -> users.js
// - /api/auth/ -> auth.js
// - /api/resources/ -> resources.js
// - /api/access/ -> access.js
// - /api/share/ (and /api/shares/) -> share.js
// - /api/settings/ -> settings.js
// - /api/tools/ -> tools.js
// - /api/office/ -> office.js
// - /public/api/* -> public.js

import * as authApi from "./auth";
import * as usersApi from "./users";
import * as resourcesApi from "./resources";
import * as accessApi from "./access";
import * as shareApi from "./share";
import * as settingsApi from "./settings";
import * as toolsApi from "./tools";
import * as officeApi from "./office";
import * as publicApi from "./public";

export { 
    authApi,
    usersApi,
    resourcesApi,
    accessApi,
    shareApi,
    settingsApi,
    toolsApi,
    officeApi,
    publicApi
};

