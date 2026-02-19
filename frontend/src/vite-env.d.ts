/// <reference types="vite/client" />

declare module '@noriginmedia/norigin-spatial-navigation' {
    export function init(options?: any): void;
    export function setFocus(focusKey: string): void;
    export function getCurrentFocusKey(): string;
    export function useFocusable(config?: any): {
        ref: any;
        focusSelf: (focusKey?: string) => void;
        focused: boolean;
        hasFocusedChild: boolean;
        focusKey: string;
    };
}
