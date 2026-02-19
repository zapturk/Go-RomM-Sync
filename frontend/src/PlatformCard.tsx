import { types } from "../wailsjs/go/models";
import { useEffect, useState } from "react";
import { GetPlatformCover } from "../wailsjs/go/main/App";
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';

interface PlatformCardProps {
    platform: types.Platform;
    onClick?: () => void;
    onEnterPress?: () => void;
}

export function PlatformCard({ platform, onClick, onEnterPress }: PlatformCardProps) {
    const [imageSrc, setImageSrc] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);

    const { ref, focused, focusSelf } = useFocusable({
        onEnterPress: onEnterPress || onClick,
        focusKey: `platform-${platform.id}`,
        onFocus: () => console.log("Platform focused:", platform.name)
    });

    useEffect(() => {
        if (!platform.slug) {
            setLoading(false);
            return;
        }

        GetPlatformCover(platform.id, platform.slug)
            .then((dataURI) => {
                if (dataURI) {
                    setImageSrc(dataURI);
                }
            })
            .catch((err) => {
                console.error("Failed to fetch platform cover:", err);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [platform.id, platform.slug]);

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
                focusSelf();
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
