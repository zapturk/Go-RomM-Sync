import { types } from "../wailsjs/go/models";
import { useEffect, useState, useRef } from "react";
import { GetPlatformCover } from "../wailsjs/go/main/App";
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { getCachedImage, setCachedImage, hasCachedImage } from './ImageCache';

interface PlatformCardProps {
    platform: types.Platform;
    onClick?: () => void;
    onEnterPress?: () => void;
    syncTrigger?: number; // Made optional to support legacy usage if any, but we pass it
    isLeftmost?: boolean;
    isTopRow?: boolean;
}

export function PlatformCard({ platform, onClick, onEnterPress, syncTrigger = 0, isLeftmost = false, isTopRow = false }: PlatformCardProps) {
    const [imageSrc, setImageSrc] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);

    // Track the last sync trigger we processed to detect changes
    const prevSyncTrigger = useRef(syncTrigger);

    const { ref, focused, focusSelf } = useFocusable({
        onEnterPress: onEnterPress || onClick,
        focusKey: `platform-${platform.id}`,
        onFocus: () => console.log("Platform focused:", platform.name),
        onArrowPress: (direction: string) => {
            if (isLeftmost && direction === 'left') {
                return false;
            }
            if (isTopRow && direction === 'up') {
                return false;
            }
            return true;
        }
    });

    useEffect(() => {
        if (!platform.slug) {
            setLoading(false);
            return;
        }

        const isSyncing = syncTrigger !== prevSyncTrigger.current;
        prevSyncTrigger.current = syncTrigger;

        // Logic Verification:
        // 1. Syncing (User pressed R/X): Always fetch fresh to check for updates.
        // 2. Cache Miss: Always fetch.
        // 3. Cache Hit & Not Syncing (Just navigating): Use Cache.

        if (!isSyncing && hasCachedImage(platform.id)) {
            setImageSrc(getCachedImage(platform.id)!);
            setLoading(false);
            return;
        }

        // Fetch from backend (Scenario 1 & 2 & Cache Miss)
        setLoading(true);
        GetPlatformCover(platform.id, platform.slug)
            .then((dataURI) => {
                if (dataURI) {
                    setImageSrc(dataURI);
                    setCachedImage(platform.id, dataURI);
                }
            })
            .catch((err) => {
                console.error("Failed to fetch platform cover:", err);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [platform.id, platform.slug, syncTrigger]);

    useEffect(() => {
        if (focused && ref.current) {
            ref.current.scrollIntoView({
                behavior: 'smooth',
                block: 'center',
            });
        }
    }, [focused]);

    return (
        <div
            ref={ref}
            className={`card game-card ${focused ? 'focused' : ''}`}
            onClick={onClick}
            onMouseEnter={() => {
                if (getMouseActive()) {
                    focusSelf();
                }
            }}
        >
            {loading ? (
                <div className="platform-image-container">
                    <div className="cover-placeholder">Loading...</div>
                </div>
            ) : imageSrc ? (
                <div className="platform-image-container">
                    <img src={imageSrc} alt={platform.name} className="platform-image" />
                </div>
            ) : (
                <div className="no-cover">No Icon</div>
            )}
            <h3>{platform.name}</h3>
        </div>
    );
}
