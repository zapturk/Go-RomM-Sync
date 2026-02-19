
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

    return (
        <div className="card game-card" onClick={onClick}>
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
