import { mutations } from "@/store";
import router from "@/router";
import { baseURL } from "@/utils/constants";

export function parseToken(token) {
  const parts = token.split(".");

  if (parts.length !== 3) {
    throw new Error("token malformed");
  }
  console.log("token")
  const data = JSON.parse(atob(parts[1]));
  document.cookie = `auth=${token}; path=/`;
  localStorage.setItem("jwt", token);
  mutations.setJWT(token);
  mutations.setSession(generateRandomCode(8));
  console.log("setting user")
  mutations.setUser(data.user);
}

export async function validateLogin() {
  try {
    if (localStorage.getItem("jwt")) {
      await renew(localStorage.getItem("jwt"));
    }
  } catch (_) {
    console.warn('Invalid JWT token in storage')
  }
}

export async function login(username, password, recaptcha) {
  const data = { username, password, recaptcha };

  const res = await fetch(`${baseURL}/api/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });
  const body = await res.text();

  if (res.status === 200) {
    parseToken(body);
  } else {
    throw new Error(body);
  }
}

export async function renew(jwt) {
  const res = await fetch(`${baseURL}/api/renew`, {
    method: "POST",
    headers: {
      "X-Auth": jwt,
    },
  });

  const body = await res.text();

  if (res.status === 200) {
    mutations.setSession(generateRandomCode(8));
    parseToken(body);
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

export async function signup(username, password) {
  const data = { username, password };

  const res = await fetch(`${baseURL}/api/signup`, {
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

  mutations.setJWT("");
  mutations.setUser(null);
  localStorage.setItem("jwt", null);
  router.push({ path: "/login" });
}
