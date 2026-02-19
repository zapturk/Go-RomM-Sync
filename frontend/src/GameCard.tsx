import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';
import { useEffect } from "react";
import { getMouseActive } from './inputMode';

interface GameCardProps {
    game: types.Game;
    onClick?: () => void;
    onEnterPress?: () => void;
}

export function GameCard({ game, onClick, onEnterPress }: GameCardProps) {
    const { ref, focused, focusSelf } = useFocusable({
        onEnterPress: onEnterPress || onClick,
        focusKey: `game-${game.id}`,
    });

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
            <GameCover game={game} className="game-cover" />
            <h3>{game.name}</h3>
        </div>
    );
}
