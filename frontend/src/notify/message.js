import { mutations, state } from "@/store";

export function showPopup(type, message) {
    const [popup, popupContent] = getElements();
    if (popup == undefined) {
        return
    }
    popup.classList.remove('success', 'error'); // Clear previous types
    popup.classList.add(type);
    popupContent.textContent = message;
    popup.style.right = '1em';

    // don't hide for actions
    if (type == "action") {
        popup.classList.add("success");
        return
    }
    // Start animation: bring the popup into view
    // Automatically hide after 10 seconds
    setTimeout(() => {
        closePopUp()
    }, 10000)
}

export function closePopUp() {
    const [popup, popupContent] = getElements();
    if (popupContent == undefined) {
        return
    }
    if (popupContent.textContent == "Multiple Selection Enabled" && state.multiple) {
        mutations.setMultiple(false)
    }
    popup.style.right = '-50em'; // Slide out
    popupContent.textContent = "no content";
}

function getElements() {
    const popup = document.getElementById('popup-notification');
    if (!popup) {
        return [null, null];
    }

    const popupContent = popup.querySelector('#popup-notification-content');
    if (!popupContent) {
       return [null, null];
    }

    return [popup, popupContent];
}

export function showSuccess(message) {
    showPopup('success', message);
}

export function showError(message) {
    showPopup('error', message);
    console.error(message)
}

export function showMultipleSelection() {
    showPopup("action","Multiple Selection Enabled");
}