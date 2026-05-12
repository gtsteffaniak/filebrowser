import * as filesApi from "./files";
import * as publicApi from "./public";
import * as usersApi from "./users";
import * as settingsApi from "./settings";
import * as accessApi from "./access";
import * as shareApi from "./share";
import * as chainfsApi from "./chainfs";
import * as safeModeApi from "./safemode";
import search from "./search";

// Note: shareApi has been consolidated into publicApi
export { filesApi, publicApi, usersApi, settingsApi, shareApi, search, accessApi, chainfsApi, safeModeApi };
