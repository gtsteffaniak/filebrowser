import type { RouteLocation, RouteRecordRaw } from "vue-router";
import { createRouter, createWebHistory } from "vue-router";

import i18n from "@/i18n";
import { getters, mutations, state } from "@/store";
import { validateLogin } from "@/utils/auth";
import { globalVars } from "@/utils/constants";
import Errors from "@/views/Errors.vue";
import Files from "@/views/Files.vue";
import Layout from "@/views/Layout.vue";
import Login from "@/views/Login.vue";
import Settings from "@/views/Settings.vue";
import Tools from "@/views/Tools.vue";

const translate = (key: string): string => i18n.global.t(key) as string;

const titles: Record<string, string> = {
  Login: translate("general.login"),
  Share: translate("general.share"),
  /** Same component as Share; name differs for /public optional-auth routes. */
  PublicShare: translate("general.share"),
  Files: translate("general.files"),
  Tools: translate("tools.title"),
  ChildTool: translate("tools.title"),
  Settings: translate("general.settings"),
  ProfileSettings: translate("settings.profileSettings"),
  Shares: translate("settings.shareManagement"),
  GlobalSettings: translate("settings.globalSettings"),
  Users: translate("general.users"),
  User: translate("general.user"),
  Forbidden: translate("errors.forbidden"),
  NotFound: translate("errors.notFound"),
  ShareNotFound: translate("errors.shareNotFound"),
  InternalServerError: translate("errors.internal"),
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
        name: "ChildTool",
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
  const toQuery = JSON.stringify(to.query);
  const fromQuery = JSON.stringify(from.query);

  return to.path === from.path &&
    to.hash === from.hash &&
    toQuery === fromQuery;
}

router.beforeResolve(async (to, from, next) => {
  if (isSameRoute(to, from)) {
    console.warn("Avoiding recursive navigation to the same route.");
    next(false);
    return;
  }

  // Clear any popup previews when navigating
  mutations.setPreviewSource("");

  const title = titles[to.name as keyof typeof titles] || String(to.name || '');
  document.title = `${globalVars.name} - ${title}`;
  mutations.setRoute(to);

  if (
    to.matched.some((record) => record.meta.requiresAuth) ||
    to.matched.some((record) => record.meta.optionalAuth)
  ) {
    const isPublicRoute = to.path.startsWith("/public");
    if (state.user?.username) {
      // do nothing, user is already set
    } else {
      try {
        await validateLogin(isPublicRoute);
      } catch (_error) {
        await mutations.setCurrentUser(getters.anonymous());
      }
    }

    if (getters.isLoggedIn() || to.matched.some((record) => record.meta.optionalAuth)) {
      // do nothing
    } else {
      // Validation failed - clear state and redirect to login
      void mutations.setCurrentUser(null);
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
