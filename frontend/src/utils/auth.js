import { mutations, getters, state } from "@/store";
import router from "@/router";
import { baseURL } from "@/utils/constants";
import { usersApi } from "@/api";

export async function setNewToken(token) {
  document.cookie = `auth=${token}; path=/`;
  mutations.setSession(generateRandomCode(8));
}

export async function validateLogin() {
  const authToken = getCookie("auth");
  try {
    let userInfo = await usersApi.get("self");
    mutations.setCurrentUser(userInfo);
  } catch {
    await renew(authToken);
  }
  return getters.isLoggedIn()
}

export async function login(username, password, recaptcha) {
  const data = { username, password, recaptcha };
  const res = await fetch(`${baseURL}/api/auth/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });
  const body = await res.text();

  if (res.status === 200) {
    await setNewToken(body);
  } else {
    throw new Error(body);
  }
}

export async function renew(jwt) {
  console.log("Renewing token");
  const res = await fetch(`${baseURL}/api/auth/renew`, {
    method: "POST",
    headers: {
      "X-Auth": jwt,
    },
  });
  const body = await res.text();
  if (res.status === 200) {
    mutations.setSession(generateRandomCode(8));
    await setNewToken(body);
  } else {
    throw new Error(body);
  }
}

function generateRandomCode(length) {
  const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let code = '';
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * charset.length);
    code += charset[randomIndex];
  }

  return code;
}

export async function signupLogin(username, password) {
  const data = { username, password };
  const res = await fetch(`${baseURL}/api/auth/signup`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  if (res.status !== 200) {
    throw new Error(res.status);
  }
}

export function logout() {
  document.cookie = "auth=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
  mutations.setCurrentUser(null);
  router.push({ path: "/login" });
}

// Helper function to retrieve the value of a specific cookie
function getCookie(name) {
  return document.cookie
    .split('; ')
    .find(row => row.startsWith(name + '='))
    ?.split('=')[1];
}