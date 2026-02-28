
import { useState, useEffect } from 'react';
import { GetCover } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";

interface GameCoverProps {
    game: types.Game;
    className?: string;
}

export function GameCover({ game, className }: GameCoverProps) {
    const [imageSrc, setImageSrc] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (!game.url_cover) {
            setLoading(false);
            return;
        }

        GetCover(game.id, game.url_cover)
            .then((dataUri) => {
                if (dataUri) {
                    setImageSrc(dataUri);
                }
            })
            .catch((err) => {
                console.error("Failed to fetch cover:", err);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [game.id, game.url_cover]);

    if (loading) {
        return <div className={`cover-placeholder ${className}`}>Loading...</div>;
    }

    if (!imageSrc) {
        return <div className={`no-cover ${className}`}>No Cover</div>;
    }

    return <img src={imageSrc} alt={game.name} className={className} />;
}
