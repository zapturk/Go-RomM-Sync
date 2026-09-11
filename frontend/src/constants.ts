/**
 * Regular expression used to clean timestamps from filenames.
 * Matches patterns like " [2023-01-01_12-00-00]" or " [2023-01-01_12-00-00-1]".
 */
export const TIMESTAMP_REGEX = / \[\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}(?:-\d+)?\]/g;

/**
 * Common application event names used with the Wails runtime.
 */
export const APP_EVENTS = {
    GAME_STARTED: 'game-started',
    GAME_EXITED: 'game-exited',
} as const;

export interface ControllerOption {
    id: string;
    name: string;
}

export const WII_CONTROLLER_OPTIONS: ControllerOption[] = [
    { id: '769', name: 'Wiimote + Nunchuk' },
    { id: '1025', name: 'Classic Controller' },
    { id: '1281', name: 'Classic Controller Pro' },
    { id: '513', name: 'Wiimote (Sideways)' },
    { id: '1', name: 'Wiimote' },
    { id: '1537', name: 'GameCube Controller' },
    { id: '2305', name: 'Wiimote + MotionPlus + Nunchuk' },
    { id: '1793', name: 'Wiimote + MotionPlus' },
];

