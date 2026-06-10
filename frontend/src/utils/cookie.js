export default function getCookieDefault(name) {
  return getCookie(name);
}

export function getCookie(name) {
  const cookie = document.cookie
    .split(";")
    .find((c) => c.trim().startsWith(`${name}=`));
  if (cookie) {
    return cookie.split("=")[1];
  }
  return ""
}
