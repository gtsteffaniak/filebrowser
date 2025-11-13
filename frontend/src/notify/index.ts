import * as messageFunctions from "./message.js";
import * as events from "./events.js";

const notify = {
    ...messageFunctions,
    closeNotification: messageFunctions.closeNotification,
    getNotifications: messageFunctions.getNotifications,
    setUpdateCallback: messageFunctions.setUpdateCallback,
    // Toast functions
    showToast: messageFunctions.showToast,
    showSuccessToast: messageFunctions.showSuccessToast,
    showErrorToast: messageFunctions.showErrorToast,
    showInfoToast: messageFunctions.showInfoToast,
    showWarningToast: messageFunctions.showWarningToast,
    getToasts: messageFunctions.getToasts,
    closeToast: messageFunctions.closeToast,
    setToastUpdateCallback: messageFunctions.setToastUpdateCallback,
};

export { notify, events };