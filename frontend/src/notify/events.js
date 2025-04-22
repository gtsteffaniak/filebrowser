import { mutations, state } from "@/store";
import { notify } from "@/notify";
import { baseURL } from "@/utils/constants";
import { filesApi } from "@/api"

async function updateSourceInfo() {
    const sourceinfo = await filesApi.sources();
    mutations.updateSourceInfo(sourceinfo);
}

export async function startSSE() {

    const eventSrc = new EventSource(`${baseURL}api/events?sessionId=${state.sessionId}`);

    eventSrc.onopen = () => {
        console.log("SSE connection established.");
        updateSourceInfo()
    };

    eventSrc.onerror = () => {
        mutations.updateSourceInfo("error");
        if (state.realtimeActive === true) {
            mutations.setRealtimeActive(false);
            notify.showError("The connection to server was lost. Trying to reconnect...");
        }
    };

    eventSrc.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            eventRouter(msg.eventType,msg.message);
            //console.log("Received event:", msg);
        } catch (err) {
            console.log("Error parsing event data:", err,event.data);
        }
    };
}


async function eventRouter(eventType,message) {
    switch (eventType) {
        case "notification":
            if (message == 'the server is shutting down') {
                notify.showError("The server was shutdown - contact your admin if this was unexpected. Trying to reconnect...");
                mutations.setRealtimeActive(false);
                return
            }
            break
        case "watchDirChange":
            mutations.setWatchDirChangeAvailable(message);
            break
        case "sourceUpdate":
            mutations.updateSourceInfo(message);
            break
        case "acknowledge":
            if (state.realtimeActive === false) {
                notify.showSuccess("The connection to server was re-established.");
            }
            mutations.setRealtimeActive(true);
            break
        default:
            console.log("something happened but don't know what", message);
            break
    }
}