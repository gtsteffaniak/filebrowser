import { createApp, nextTick } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createUserEditSession, destroyUserEditSession, getUserEditSession } from "@/utils/userEditSession.js";
import { applySectionsToFlatUser, sectionsFromFlatUser } from "@/utils/userProfileSections.js";

const { fetchURL, fetchJSON, state, mutations } = vi.hoisted(() => ({
  fetchURL: vi.fn(),
  fetchJSON: vi.fn(),
  state: { user: { username: "admin" }, enforcedUserDefaults: {} },
  mutations: { closeTopPrompt: vi.fn() },
}));
vi.mock("@/api", async () => ({
  usersApi: await vi.importActual("@/api/users.js"),
  settingsApi: {},
  authApi: {},
  accessApi: {},
}));
vi.mock("@/api/utils.ts", () => ({ fetchURL, fetchJSON }));
vi.mock("@/router/index.ts", () => ({ router: {} }));
vi.mock("@/store", () => ({ state, mutations }));
vi.mock("@/store/state.ts", () => ({ state }));
vi.mock("@/store/mutations.ts", () => ({ mutations }));
vi.mock("@/store/eventBus", () => ({ eventBus: { emit: vi.fn() } }));
vi.mock("@/notify", () => ({ notify: { showSuccessToast: vi.fn(), showError: vi.fn() } }));
vi.mock("@/i18n/index.ts", () => ({ default: { global: { t: (key) => key } } }));
vi.mock("@/utils/auth", () => ({ validateLogin: vi.fn() }));
vi.mock("@/components/settings/UserProfilePreferences.vue", () => ({ default: { render: () => null } }));

import { get } from "@/api/users.js";
import UserEdit from "./UserEdit.vue";
import UserEditPreferences from "./UserEditPreferences.vue";

let app, root;
function openPreferences(user) {
  createUserEditSession({ user, profileUser: { ...user } });
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(UserEditPreferences);
  app.config.globalProperties.$t = (key) => key;
  app.mount(root);
}

function passwordChangeToggle() {
  return root.querySelector('input[aria-label="settings.requirePasswordChange"]');
}

afterEach(() => {
  app?.unmount();
  app = undefined;
  root?.remove();
  destroyUserEditSession();
  state.enforcedUserDefaults = {};
  fetchURL.mockReset();
  fetchJSON.mockReset();
});

describe("password changes in user edit preferences", () => {
  it.each([false, true])("passes the changed requirement to the parent save (initially %s)", async (initial) => {
    const user = { username: "user", loginMethod: "password", requirePasswordChange: initial };
    applySectionsToFlatUser(user, sectionsFromFlatUser(user));
    const editor = {
      ...UserEdit.data(),
      user: { ...user },
      profileUser: { ...user },
      loaded: true,
      groupsReady: true,
      isNew: false,
      enforcedAccount: {},
      enforcedAccountPermissions: {},
      enforcedPreferences: {},
      $t: (key) => key,
    };
    for (const [name, method] of Object.entries(UserEdit.methods)) {
      editor[name] = method.bind(editor);
    }
    editor.saveGroups = vi.fn();
    editor.syncEditAccountForm();
    editor.originalSnapshot = editor.buildEditableSnapshot();

    openPreferences(user);
    await nextTick();
    const toggle = passwordChangeToggle();
    expect(toggle?.checked).toBe(initial);
    toggle.click();
    await nextTick();
    expect(getUserEditSession().user.requirePasswordChange).toBe(!initial);
    expect(getUserEditSession().profileUser.requirePasswordChange).toBe(!initial);

    await editor.save({ preventDefault: vi.fn() });
    expect(fetchURL).toHaveBeenCalledTimes(1);
    const [url, request] = fetchURL.mock.calls[0];
    expect(url).toMatch(/\/api\/users\?username=user$/);
    expect(request.method).toBe("PATCH");
    const body = JSON.parse(request.body);
    expect(body).toEqual({
      which: ["requirePasswordChange"],
      data: { requirePasswordChange: !initial },
    });

    app.unmount();
    root.remove();
    fetchJSON.mockResolvedValue({ ...user, ...body.data });
    openPreferences(await get(user.username));
    await nextTick();
    expect(passwordChangeToggle()?.checked).toBe(!initial);
  });

  it("hides the toggle for externally authenticated users", async () => {
    openPreferences({ username: "user", loginMethod: "oidc" });
    await nextTick();
    expect(passwordChangeToggle()).toBeNull();
  });

  it("disables an enforced requirement", async () => {
    state.enforcedUserDefaults = { account: { requirePasswordChange: true } };
    openPreferences({ username: "user", loginMethod: "password", requirePasswordChange: true });
    await nextTick();
    expect(passwordChangeToggle()?.disabled).toBe(true);
  });
});
