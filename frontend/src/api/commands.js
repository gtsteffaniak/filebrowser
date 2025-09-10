import { globalVars } from "@/utils/constants";

const ssl = window.location.protocol === "https:";
const protocol = ssl ? "wss:" : "ws:";

export default function command(url, command, onmessage, onclose) {
  url = `${protocol}//${window.location.host}${globalVars.baseURL}api/command${url}`;
  let conn = new window.WebSocket(url);
  conn.onopen = () => conn.send(command);
  conn.onmessage = onmessage;
  conn.onclose = onclose;
}
