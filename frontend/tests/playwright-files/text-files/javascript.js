// Simple JavaScript file
const elements = document.querySelectorAll('.item');
for (let i = 0; i < elements.length; i++) {
    const element = elements[i];
    if (i % 2 === 0) {
        element.classList.add('even');
        element.classList.remove('odd');
    } else {
        element.classList.add('odd');
        element.classList.remove('even');
    }
    if (element.classList.contains('active')) {
        element.style.backgroundColor = '#4CAF50';
    } else {
        element.style.backgroundColor = '#f0f0f0';
    }
}
