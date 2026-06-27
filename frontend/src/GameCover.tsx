
import { useState, useEffect } from "react";
import { types } from "../wailsjs/go/models";

interface GameCoverProps {
    game: types.Game;
    className?: string;
}

export function GameCover({ game, className }: GameCoverProps) {
    const [hasError, setHasError] = useState(false);

    const src = game.url_cover ? `/cache/covers/${game.id}.jpg?url=${encodeURIComponent(game.url_cover)}` : "";

    // Reset the error state if the cover image source changes
    useEffect(() => {
        setHasError(false);
    }, [src]);

    if (!game.url_cover || hasError) {
        return <div className={`no-cover ${className}`}>No Cover</div>;
    }

    return (
        <div className={`game-cover ${className}`} style={{ position: 'relative' }}>
            <img
                src={src}
                alt={game.name}
                style={{ width: '100%', height: '100%', objectFit: 'cover', borderRadius: '4px' }}
                loading="lazy"
                onError={() => setHasError(true)}
            />
        </div>
    );
}

