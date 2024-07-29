export function showPopup(type, message) {
    const popup = document.getElementById('popup-notification');
    if (popup === null) {
        console.error('Popup notification :',type,message);
        return;
    }
    popup.classList.add(type);
    popup.textContent = message;
    popup.style.display = 'block'; // Make it visible

    // Automatically hide after 3 seconds
    setTimeout(() => {
        popup.classList.remove(type);
        popup.style.display = 'none';
    }, 3000);
}

export function showSuccess(message) {
    showPopup('success', message);
}

export function showError(message) {
    showPopup('error', message);
    console.error(message)
}