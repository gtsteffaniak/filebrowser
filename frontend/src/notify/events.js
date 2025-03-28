import { mutations,getters,state } from "@/store";

export async function startSSE() {
    if (!getters.isLoggedIn()) {
        return;
    }
    if (!state.user.perm.realtime) {
        return;
    }
    const eventSrc = new EventSource(`/api/events?sessionId=${state.sessionId}`);

    eventSrc.onopen = () => {
        console.log("SSE connection established.");
    };

    eventSrc.onerror = (err) => {
        mutations.updateSourceInfo("error");
        console.log("SSE error:", err);
    };

    eventSrc.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            eventRouter(msg.eventType,msg.message);
            console.log("Received event:", msg);
        } catch (err) {
            console.log("Error parsing event data:", err,event.data);
        }
    };
}


async function eventRouter(eventType,message) {
    switch (eventType) {
        case "sourceUpdate":
            mutations.updateSourceInfo(message);
            break
        case "acknowledge":
            mutations.setRealtimeActive(message);
            break
        default:
            console.log("something happened but don't know what", message);
            break
    }
}