import { RouteLocation, createRouter, createWebHistory } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Share from "@/views/Share.vue";
import Settings from "@/views/Settings.vue";
import Errors from "@/views/Errors.vue";
import { baseURL, name } from "@/utils/constants";
import { getters, state } from "@/store";
import { mutations } from "@/store";
import { validateLogin } from "@/utils/auth";
import i18n from "@/i18n";

const titles = {
  Login: "sidebar.login",
  Share: "buttons.share",
  Files: "files.files",
  Settings: "sidebar.settings",
  ProfileSettings: "settings.profileSettings",
  Shares: "settings.shareManagement",
  GlobalSettings: "settings.globalSettings",
  Users: "settings.users",
  User: "settings.user",
  Forbidden: "errors.forbidden",
  NotFound: "errors.notFound",
  InternalServerError: "errors.internal",
};

const routes = [
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
        component: Share,
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
  history: createWebHistory(baseURL),
  routes,
});

// Helper function to check if a route resolves to itself
function isSameRoute(to: RouteLocation, from: RouteLocation) {
  return to.path === from.path && JSON.stringify(to.params) === JSON.stringify(from.params) && to.hash === from.hash;
}

router.beforeResolve(async (to, from, next) => {
  if (isSameRoute(to, from)) {
    console.warn("Avoiding recursive navigation to the same route.");
    return next(false);
  }

  // Set the page title using i18n
  const title = i18n.global.t(titles[to.name as keyof typeof titles]);
  document.title = title + " - " + name;

  // Update store with the current route
  mutations.setRoute(to);

  // Handle auth requirements
  if (to.matched.some((record) => record.meta.requiresAuth)) {

    if (state != null && state.user != null && !('username' in state.user)) {
      await validateLogin();
    }

    if (!getters.isLoggedIn()) {
      next({
        path: "/login",
        query: { redirect: to.fullPath },
      });
      return;
    }

    // Handle admin-only routes
    if (to.matched.some((record) => record.meta.requiresAdmin)) {
      if (!state.user || !getters.isAdmin()) {
        next({ path: "/403" });
        return;
      }
    }
  }

  // Redirect logged-in users from login page
  if (to.path.endsWith("/login") && getters.isLoggedIn()) {
    next({ path: "/files/" });
    return;
  }

  next();
});

export { router, router as default };
