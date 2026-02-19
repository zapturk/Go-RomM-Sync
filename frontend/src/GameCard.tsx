
import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";

interface GameCardProps {
    game: types.Game;
    onClick?: () => void;
}

export function GameCard({ game, onClick }: GameCardProps) {
    return (
        <div className="card game-card" onClick={onClick}>
            <GameCover game={game} className="game-cover" />
            <h3>{game.name}</h3>
        </div>
    );
}
