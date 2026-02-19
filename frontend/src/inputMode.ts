// Simple global state to track if the user is currently using the mouse.
// This helps prevent "focus stealing" when the view scrolls under a stationary mouse cursor
// while using keyboard/gamepad navigation.

let isMouseActive = false;

if (typeof window !== 'undefined') {
    window.addEventListener('mousemove', (e) => {
        // Only consider the mouse active if it actually moved.
        // Browsers sometimes fire mousemove on scroll even if the physical mouse didn't move.
        // We use a small threshold or check movementX/Y to be sure.
        if (Math.abs(e.movementX) > 0 || Math.abs(e.movementY) > 0) {
            isMouseActive = true;
            document.body.classList.add('mouse-mode');
            document.body.classList.remove('keyboard-mode');
        }
    });

    window.addEventListener('keydown', () => {
        isMouseActive = false;
        document.body.classList.remove('mouse-mode');
        document.body.classList.add('keyboard-mode');
    });

    // Also listen for mousedown just in case
    window.addEventListener('mousedown', () => {
        isMouseActive = true;
        document.body.classList.add('mouse-mode');
        document.body.classList.remove('keyboard-mode');
    });
}

export const getMouseActive = () => isMouseActive;
