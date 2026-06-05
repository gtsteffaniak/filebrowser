export default function (name) {
  const re = new RegExp(
      `(?:(?:^|.*;\\s*)${name}\\s*\\=\\s*([^;]*).*$)|^.*$`
  );
  return document.cookie.replace(re, "$1");
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
