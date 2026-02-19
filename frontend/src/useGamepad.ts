import { useEffect, useRef } from 'react';

// Map for standard controllers (Xbox/PlayStation via standard gamepad API)
const BUTTON_MAPPING = {
    A: 0,
    B: 1,
    X: 2,
    Y: 3,
    LB: 4,
    RB: 5,
    LT: 6,
    RT: 7,
    BACK: 8,
    START: 9,
    LSTICK: 10,
    RSTICK: 11,
    DPAD_UP: 12,
    DPAD_DOWN: 13,
    DPAD_LEFT: 14,
    DPAD_RIGHT: 15,
};

// Threshold for stick movement
const STICK_THRESHOLD = 0.5;
// Delay between inputs in ms
const INPUT_DELAY = 150;

export function useGamepad() {
    const lastInputTime = useRef(0);
    const requestRef = useRef<number>();

    const triggerKey = (key: string) => {
        const now = Date.now();
        if (now - lastInputTime.current < INPUT_DELAY) return;

        lastInputTime.current = now;
        window.dispatchEvent(new KeyboardEvent('keydown', { code: key, key: key, bubbles: true }));
    };

    const scanGamepads = () => {
        const gamepads = navigator.getGamepads();

        for (const gp of gamepads) {
            if (!gp) continue;

            // D-PAD
            if (gp.buttons[BUTTON_MAPPING.DPAD_UP]?.pressed) triggerKey('ArrowUp');
            if (gp.buttons[BUTTON_MAPPING.DPAD_DOWN]?.pressed) triggerKey('ArrowDown');
            if (gp.buttons[BUTTON_MAPPING.DPAD_LEFT]?.pressed) triggerKey('ArrowLeft');
            if (gp.buttons[BUTTON_MAPPING.DPAD_RIGHT]?.pressed) triggerKey('ArrowRight');

            // Left Stick
            if (gp.axes[1] < -STICK_THRESHOLD) triggerKey('ArrowUp');
            if (gp.axes[1] > STICK_THRESHOLD) triggerKey('ArrowDown');
            if (gp.axes[0] < -STICK_THRESHOLD) triggerKey('ArrowLeft');
            if (gp.axes[0] > STICK_THRESHOLD) triggerKey('ArrowRight');

            // Actions
            if (gp.buttons[BUTTON_MAPPING.A]?.pressed) triggerKey('Enter');
            if (gp.buttons[BUTTON_MAPPING.B]?.pressed) triggerKey('Backspace'); // Or Escape
            if (gp.buttons[BUTTON_MAPPING.X]?.pressed) triggerKey('r'); // Refresh/Sync
        }

        requestRef.current = requestAnimationFrame(scanGamepads);
    };

    useEffect(() => {
        window.addEventListener("gamepadconnected", () => {
            console.log("Gamepad connected!");
        });

        requestRef.current = requestAnimationFrame(scanGamepads);

        return () => {
            if (requestRef.current) cancelAnimationFrame(requestRef.current);
        };
    }, []);
}
