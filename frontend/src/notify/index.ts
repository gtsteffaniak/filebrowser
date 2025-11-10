import * as messageFunctions from "./message.js";
import * as events from "./events.js";

const notify = {
    ...messageFunctions,
    closeNotification: messageFunctions.closeNotification,
    getNotifications: messageFunctions.getNotifications,
    setUpdateCallback: messageFunctions.setUpdateCallback,
};

export { notify, events };