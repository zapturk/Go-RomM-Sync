
import { types } from "../wailsjs/go/models";
import { useEffect, useState } from "react";
import { GetPlatformCover } from "../wailsjs/go/main/App";

interface PlatformCardProps {
    platform: types.Platform;
    onClick?: () => void;
}

export function PlatformCard({ platform, onClick }: PlatformCardProps) {
    const [imageSrc, setImageSrc] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (!platform.url_icon) {
            setLoading(false);
            return;
        }

        GetPlatformCover(platform.id, platform.url_icon)
            .then((base64) => {
                if (base64) {
                    setImageSrc(`data:image/jpeg;base64,${base64}`);
                }
            })
            .catch((err) => {
                console.error("Failed to fetch platform cover:", err);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [platform.id, platform.url_icon]);

    return (
        <div className="card game-card" onClick={onClick}>
            {loading ? (
                <div className="cover-placeholder game-cover">Loading...</div>
            ) : imageSrc ? (
                <img src={imageSrc} alt={platform.name} className="game-cover" />
            ) : (
                <div className="no-cover">No Icon</div>
            )}
            <h3>{platform.name}</h3>
        </div>
    );
}
