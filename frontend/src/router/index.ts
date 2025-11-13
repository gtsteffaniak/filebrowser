import { RouteLocation, createRouter, createWebHistory, RouteRecordRaw } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Settings from "@/views/Settings.vue";
import Notifications from "@/views/Notifications.vue";
import Errors from "@/views/Errors.vue";
import Tools from "@/views/Tools.vue";
import { globalVars } from "@/utils/constants";
import { getters, state } from "@/store";
import { mutations } from "@/store";
import { validateLogin } from "@/utils/auth";
import i18n from "@/i18n";

const titles = {
  Login: i18n.global.t("general.login"),
  Share: i18n.global.t("general.share"),
  PublicShare: i18n.global.t("general.share"),
  Files: i18n.global.t("general.files"),
  Tools: i18n.global.t("general.tool"),
  Settings: i18n.global.t("general.settings"),
  Notifications: i18n.global.t("notifications.title"),
  ProfileSettings: i18n.global.t("settings.profileSettings"),
  Shares: i18n.global.t("settings.shareManagement"),
  GlobalSettings: i18n.global.t("settings.globalSettings"),
  Users: i18n.global.t("general.users"),
  User: i18n.global.t("general.user"),
  Forbidden: i18n.global.t("errors.forbidden"),
  NotFound: i18n.global.t("errors.notFound"),
  ShareNotFound: i18n.global.t("errors.shareNotFound"),
  InternalServerError: i18n.global.t("errors.internal"),
};

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "Login",
    component: Login,
  },
  {
    path: "/share",
    component: Layout,
    children: [
      {
        path: ":path*",
        name: "Share",
        component: Files,
      },
    ],
  },
  {
    path: "/public",
    component: Layout,
    meta: {
      optionalAuth: true,
    },
    children: [
      {
        path: ":path*",
        name: "PublicShare",
        component: Files,
      },
    ],
  },
  {
    path: "/files",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: ":path*",
        name: "Files",
        component: Files,
      },
    ],
  },
  {
    path: "/tools/:toolName?",
    name: "Tools",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "ToolsIndex",
        component: Tools,
      },
    ],
  },
  {
    path: "/settings",
    component: Layout,
    meta: {
      requiresAuth: true,
      requireSettingsEnabled: true
    },
    children: [
      {
        path: "",
        name: "Settings",
        component: Settings,
      },
      {
        path: "users/:id",
        name: "User",
        component: Settings,
      },
    ],
  },
  {
    path: "/notifications",
    name: "Notifications",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "NotificationsIndex",
        component: Notifications,
      },
    ],
  },
  {
    path: "/403",
    name: "Forbidden",
    component: Errors,
    props: {
      errorCode: 403,
      showHeader: true,
    },
  },
  {
    path: "/404",
    name: "NotFound",
    component: Errors,
    props: {
      errorCode: 404,
      showHeader: true,
    },
  },
  {
    path: "/500",
    name: "InternalServerError",
    component: Errors,
    props: {
      errorCode: 500,
      showHeader: true,
    },
  },
  {
    path: "/:catchAll(.*)*",
    redirect: (to: RouteLocation) => {
      const path = Array.isArray(to.params.catchAll)
        ? to.params.catchAll.join("/")
        : to.params.catchAll || "";
      return `/files/${path}`;
    },
  },
];

const router = createRouter({
  history: createWebHistory(globalVars.baseURL),
  routes,
});

// Helper function to check if a route resolves to itself
function isSameRoute(to: RouteLocation, from: RouteLocation) {
  // Allow query parameter changes - they don't count as same route
  const toQuery = JSON.stringify(to.query || {});
  const fromQuery = JSON.stringify(from.query || {});

  return to.path === from.path &&
    to.hash === from.hash &&
    toQuery === fromQuery;
}

router.beforeResolve(async (to, from, next) => {
  if (isSameRoute(to, from)) {
    console.warn("Avoiding recursive navigation to the same route.");
    return next(false);
  }

  const title = titles[to.name as keyof typeof titles] || String(to.name || '');
  document.title = globalVars.name + " - " + title;
  mutations.setRoute(to);

  if (
    to.matched.some((record) => record.meta.requiresAuth) ||
    to.matched.some((record) => record.meta.optionalAuth)
  ) {
    if (state?.user?.username) {
      // do nothing, user is already set
    } else {
      try {
        await validateLogin();
      } catch (error) {
        mutations.setCurrentUser(getters.anonymous());
      }
    }

    if (getters.isLoggedIn() || to.matched.some((record) => record.meta.optionalAuth)) {
      // do nothing
    } else {
      // Validation failed - clear state and redirect to login
      mutations.setCurrentUser(null);
      // Always redirect to login when not authenticated
      next({ path: "/login", query: { redirect: to.fullPath } });
      return;
    }

    if (to.matched.some((record) => record.meta.requiresAdmin)) {
      if (!getters.isAdmin()) {
        next({ path: "/403" });
        return;
      }
    }

    if (to.matched.some((record) => record.meta.requireSettingsEnabled)) {
      if (state.user?.disableSettings) {
        next({ path: "/files/" });
        return;
      }
    }
  }

  if (to.path.endsWith("/login") && getters.isLoggedIn()) {
    next({ path: "/files/" });
    return;
  }

  next();
});

router.afterEach((to) => {
  // Only postMessage if running in an iframe
  if (window.self !== window.top) {
    window.parent.postMessage(
      {
        type: "filebrowser:navigation",
        url: to.fullPath,
      },
      "*"
    );
  }
});

export { router, router as default };
