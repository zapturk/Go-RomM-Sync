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
