import { RouteLocation, createRouter, createWebHistory } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Share from "@/views/Share.vue";
import Settings from "@/views/Settings.vue";
import Errors from "@/views/Errors.vue";
import { baseURL, name } from "@/utils/constants";
import { getters, state } from "@/store";
import { recaptcha, loginPage } from "@/utils/constants";
import { login, validateLogin } from "@/utils/auth";
import { mutations } from "@/store";
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
        meta: {
          requiresAdmin: true,
        },
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
    redirect: (to: RouteLocation) =>
      `/files/${[...to.params.catchAll].join("/")}`,
  },
];

const router = createRouter({
  history: createWebHistory(baseURL),
  routes,
});


async function initAuth() {
  if (loginPage) {
    console.log("login page");
      await validateLogin();
  } else {
    console.log("not login page");

      await login("publicUser", "publicUser", "");
  }
  if (recaptcha) {
      await new Promise<void>((resolve) => {
          const check = () => {
              if (typeof window.grecaptcha === "undefined") {
                  setTimeout(check, 100);
              } else {
                  resolve();
              }
          };
          check();
      });
  }
}

router.beforeResolve(async (to, from, next) => {
  mutations.closeHovers()
  const title = i18n.global.t(titles[to.name as keyof typeof titles]);
  document.title = title + " - " + name;
  console.log("from",from,"to",to)
  mutations.setRoute(to)
  // this will only be null on first route
  if (from.name == null) {
    try {
      console.log("init auth")
      await initAuth();
    } catch (error) {
      console.log("saw error",error)
      console.error(error);
    }
  }
  if (to.path.endsWith("/login") && getters.isLoggedIn()) {
    console.log("redirecting to files, logged in")
    next({ path: "/files/" });
    return;
  }

  if (to.matched.some((record) => record.meta.requiresAuth)) {
    if (!getters.isLoggedIn()) {
      console.log("redirecting to login, not logged in")

      next({
        path: "/login",
        query: { redirect: to.fullPath },
      });
      return;
    }

    if (to.matched.some((record) => record.meta.requiresAdmin)) {
      if (state.user === null || !getters.isAdmin()) {
        next({ path: "/403" });
        return;
      }
    }
  }

  next();
});

export { router, router as default };