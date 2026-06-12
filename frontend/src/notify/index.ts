import * as events from "./events.js";
import * as messageFunctions from "./message.js";

const notify = {
    ...messageFunctions,
    closeNotification: messageFunctions.closeNotification,
    getNotifications: messageFunctions.getNotifications,
    setUpdateCallback: messageFunctions.setUpdateCallback,
    pauseAutoClose: messageFunctions.pauseAutoClose,
    resumeAutoClose: messageFunctions.resumeAutoClose,
    getNotificationProgress: messageFunctions.getNotificationProgress,
    resolveHistoryNotificationButtons: messageFunctions.resolveHistoryNotificationButtons,
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

export { events, notify };
