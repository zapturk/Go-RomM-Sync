import { useEffect, useRef } from 'react';
import { Quit } from "../wailsjs/go/main/App";

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
    const prevButtonsRef = useRef<Record<number, boolean>>({});
    const lastInputTime = useRef(0);
    const requestRef = useRef<number>();
    const isPlayingRef = useRef(false);

    const triggerKey = (key: string) => {
        const now = Date.now();
        if (now - lastInputTime.current < INPUT_DELAY) return;

        lastInputTime.current = now;
        window.dispatchEvent(new KeyboardEvent('keydown', { code: key, key: key, bubbles: true }));
    };

    const scanGamepads = () => {
        if (isPlayingRef.current) {
            requestRef.current = requestAnimationFrame(scanGamepads);
            return;
        }

        const gamepads = navigator.getGamepads();

        for (const gp of gamepads) {
            if (!gp) continue;

            const prevButtons = prevButtonsRef.current;
            const currentButtons: Record<number, boolean> = {};

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

            // Actions - Trigger on Button Up
            const ACTION_BUTTONS = {
                [BUTTON_MAPPING.A]: 'Enter',
                [BUTTON_MAPPING.B]: 'Escape',
                [BUTTON_MAPPING.X]: 'r',
                [BUTTON_MAPPING.LB]: 'PageUp',
                [BUTTON_MAPPING.RB]: 'PageDown',
            };

            for (const [btnIdStr, key] of Object.entries(ACTION_BUTTONS)) {
                const btnId = parseInt(btnIdStr);
                const isPressed = gp.buttons[btnId]?.pressed || false;
                currentButtons[btnId] = isPressed;

                // If it was pressed before and is now released
                if (prevButtons[btnId] && !isPressed) {
                    triggerKey(key);
                }
            }

            // Exit (Start + Select/Back)
            if (gp.buttons[BUTTON_MAPPING.START]?.pressed && gp.buttons[BUTTON_MAPPING.BACK]?.pressed) {
                Quit();
            }

            // Update stored states
            prevButtonsRef.current = currentButtons;
        }

        requestRef.current = requestAnimationFrame(scanGamepads);
    };

    useEffect(() => {
        window.addEventListener("gamepadconnected", () => {
            console.log("Gamepad connected!");
        });

        // @ts-ignore
        if (window.runtime) {
            // @ts-ignore
            window.runtime.EventsOn("game-started", () => {
                isPlayingRef.current = true;
            });
            // @ts-ignore
            window.runtime.EventsOn("game-exited", () => {
                isPlayingRef.current = false;
            });
        }

        requestRef.current = requestAnimationFrame(scanGamepads);

        return () => {
            if (requestRef.current) cancelAnimationFrame(requestRef.current);
            // @ts-ignore
            if (window.runtime) {
                // @ts-ignore
                window.runtime.EventsOff("game-started");
                // @ts-ignore
                window.runtime.EventsOff("game-exited");
            }
        };
    }, []);
}
