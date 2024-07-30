export function showPopup(type, message) {
    const [popup, popupContent] = getElements();
    popup.classList.remove('success', 'error'); // Clear previous types
    popup.classList.add(type);
    popupContent.textContent = message;

    // Start animation: bring the popup into view
    popup.style.right = '1em';

    // Automatically hide after 10 seconds
    setTimeout(() => {
        closePopUp()
    }, 10000);
}

export function closePopUp() {
    const [popup, popupContent] = getElements();
    popup.style.right = '-50em'; // Slide out
    popupContent.textContent = "no content";
}

function getElements() {
    const popup = document.getElementById('popup-notification');
    if (!popup) {
        console.error('Popup notification element not found');
        return [null, null];
    }

    const popupContent = popup.querySelector('#popup-notification-content');
    if (!popupContent) {
        console.error('Popup notification content element not found');
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