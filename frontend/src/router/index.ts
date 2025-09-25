import { RouteLocation, createRouter, createWebHistory, RouteRecordRaw } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Settings from "@/views/Settings.vue";
import Errors from "@/views/Errors.vue";
import { globalVars } from "@/utils/constants";
import { getters, state } from "@/store";
import { mutations } from "@/store";
import { validateLogin } from "@/utils/auth";
import { removeLeadingSlash } from "@/utils/url";
import i18n from "@/i18n";

const titles = {
  Login: "sidebar.login",
  Share: "buttons.share",
  PublicShare: "buttons.share",
  Files: "general.files",
  Settings: "sidebar.settings",
  ProfileSettings: "settings.profileSettings",
  Shares: "settings.shareManagement",
  GlobalSettings: "settings.globalSettings",
  Users: "settings.users",
  User: "settings.user",
  Forbidden: "errors.forbidden",
  NotFound: "errors.notFound",
  ShareNotFound: "errors.shareNotFound",
  InternalServerError: "errors.internal",
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
  return to.path === from.path && to.hash === from.hash;
}

router.beforeResolve(async (to, from, next) => {
  if (isSameRoute(to, from)) {
    console.warn("Avoiding recursive navigation to the same route.");
    return next(false);
  }

  // @ts-ignore - Temporary fix for type instantiation issue
  const title = i18n.global.t(titles[to.name as keyof typeof titles]);
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
      if (globalVars.passwordAvailable) {
        next({ path: "/login", query: { redirect: to.fullPath } });
        return;
      }

      if (globalVars.oidcAvailable) {
        const modifiedPath = encodeURIComponent(globalVars.baseURL+removeLeadingSlash(to.fullPath))
        window.location.href = globalVars.baseURL+`api/auth/oidc/login?redirect=${modifiedPath}`;
        return;
      }
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
