// Simple in-memory cache to store platform cover images (base64 strings)
// to prevent re-fetching from the backend on every component mount.

const imageCache = new Map<string, string>();

export const getCachedImage = (id: number): string | undefined => {
    return imageCache.get(id.toString());
};

export const setCachedImage = (id: number, data: string) => {
    imageCache.set(id.toString(), data);
};

export const hasCachedImage = (id: number): boolean => {
    return imageCache.has(id.toString());
};
