
import { types } from "../wailsjs/go/models";

interface GameCoverProps {
    game: types.Game;
    className?: string;
}

export function GameCover({ game, className }: GameCoverProps) {
    if (!game.url_cover) {
        return <div className={`no-cover ${className}`}>No Cover</div>;
    }

    const src = `/cache/covers/${game.id}.jpg?url=${encodeURIComponent(game.url_cover)}`;

    return (
        <div className={`game-cover-wrapper ${className}`} style={{ position: 'relative' }}>
            <img 
                src={src} 
                alt={game.name} 
                className={className}
                style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                loading="lazy"
            />
        </div>
    );
}
