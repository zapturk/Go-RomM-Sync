import { Events } from "@wailsio/runtime";

export function EventsOn<T = any>(eventName: string, callback: (data: T) => void): () => void {
    return Events.On(eventName, (event: Events.WailsEvent) => {
        callback(event.data as T);
    });
}
