export default function (name) {
  let re = new RegExp(
    "(?:(?:^|.*;\\s*)" + name + "\\s*\\=\\s*([^;]*).*$)|^.*$"
  );
  return document.cookie.replace(re, "$1");
}

export function getCookie(name) {
  let cookie = document.cookie
    .split(";")
    .find((cookie) => cookie.includes(name + "="));
  if (cookie != null) {
    return cookie.split("=")[1];
  }
  return ""
}