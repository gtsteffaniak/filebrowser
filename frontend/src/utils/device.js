export function isMobileSafari() {
  const userAgent = window.navigator.userAgent;
  const isIOS = /iPad|iPhone|iPod/.test(userAgent) && !window.MSStream;
  const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
  return isIOS && isSafari;
}