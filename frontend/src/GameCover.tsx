
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
        <div className={`game-cover ${className}`} style={{ position: 'relative' }}>
            <img
                src={src}
                alt={game.name}
                style={{ width: '100%', height: '100%', objectFit: 'cover', borderRadius: '4px' }}
                loading="lazy"
                onError={(e) => {
                    (e.target as HTMLImageElement).style.display = 'none';
                    const parent = (e.target as HTMLImageElement).parentElement as HTMLElement;
                    if (parent) {
                        const placeholder = document.createElement('div');
                        placeholder.className = `no-cover`;
                        placeholder.textContent = 'No Cover';
                        parent.appendChild(placeholder);
                    }
                }}
            />
        </div>
    );
}
