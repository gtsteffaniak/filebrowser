import { mutations } from "@/store";

export async function startSSE() {
    localStorage.getItem("sessionId");
    const eventSrc = new EventSource(`/api/events?sessionId=`);

    eventSrc.onopen = () => {
        console.log("SSE connection established.");
    };

    eventSrc.onerror = (err) => {
        console.log("SSE error:", err);
    };

    eventSrc.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            eventRouter(msg.eventType,msg.message);
            console.log("Received event:", msg);
        } catch (err) {
            console.log("Error parsing event data:", err);
        }
    };
}


async function eventRouter(eventType,message) {
    switch (eventType) {
        case "sources":
            mutations.updateSourcesStatus(message);
            break
        default:
            console.log("something happened but don't know what", message);
            break
    }
}