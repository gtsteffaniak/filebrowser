import { mutations } from "@/store";
import router from "@/router";
import { baseURL } from "@/utils/constants";

export function parseToken(token) {
  const parts = token.split(".");

  if (parts.length !== 3) {
    throw new Error("token malformed");
  }
  const data = JSON.parse(atob(parts[1]));
  document.cookie = `auth=${token}; path=/`;
  mutations.setJWT(token);
  mutations.setSession(generateRandomCode(8));
  mutations.setCurrentUser(data.user);
}

export async function validateLogin() {
  const authToken = getCookie("auth");
  if (authToken != undefined) {
    console.log("token", authToken);
    await renew(authToken);
  } else {
    console.log("No token found");
  }
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
    parseToken(body);
  } else {
    throw new Error(body);
  }
}

export async function renew(jwt) {
  const res = await fetch(`${baseURL}/api/auth/renew`, {
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
  mutations.setJWT("");
  mutations.setCurrentUser(null);
  localStorage.setItem("jwt", null);
  router.push({ path: "/login" });
}

// Helper function to retrieve the value of a specific cookie
function getCookie(name) {
  return document.cookie
    .split('; ')
    .find(row => row.startsWith(name + '='))
    ?.split('=')[1];
}