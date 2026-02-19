// Global state tracking
let currentMode: 'mouse' | 'keyboard' | 'gamepad' = 'mouse';

const setMode = (mode: 'mouse' | 'keyboard' | 'gamepad') => {
    if (currentMode === mode) return;
    currentMode = mode;
    document.body.classList.remove('input-mouse', 'input-keyboard', 'input-gamepad');
    document.body.classList.add(`input-${mode}`);

    // Legacy support for hover effects
    if (mode === 'mouse') {
        document.body.classList.add('mouse-mode');
    } else {
        document.body.classList.remove('mouse-mode');
    }
};

if (typeof window !== 'undefined') {
    // Set initial default
    document.body.classList.add('input-mouse', 'mouse-mode');

    window.addEventListener('mousemove', (e) => {
        if (Math.abs(e.movementX) > 0 || Math.abs(e.movementY) > 0) {
            setMode('mouse');
        }
    });

    window.addEventListener('keydown', (e) => {
        // e.isTrusted is true for physical key strokes, false for script-generated (gamepad)
        if (e.isTrusted) {
            setMode('keyboard');
        } else {
            setMode('gamepad');
        }
    });

    window.addEventListener('mousedown', () => {
        setMode('mouse');
    });
}

export const getMouseActive = () => currentMode === 'mouse';
export const getInputMode = () => currentMode;
