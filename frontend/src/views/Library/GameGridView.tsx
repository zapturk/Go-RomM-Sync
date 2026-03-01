import { useRef, useEffect } from 'react';
import { types } from "../../../wailsjs/go/models";
import { GameCard } from "../../GameCard";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from '../../inputMode';
import { FocusableButton } from '../../components/FocusableButton';

interface GameGridViewProps {
    platform: types.Platform;
    games: types.Game[];
    isLoading: boolean;
    offset: number;
    totalGames: number;
    pageSize: number;
    columns: number;
    lastViewedGameId: number | null;
    onSelectGame: (game: types.Game) => void;
    onPageChange: (newOffset: number) => void;
    gridRef: React.RefObject<HTMLDivElement>;
}

export function GameGridView({
    platform,
    games,
    isLoading,
    offset,
    totalGames,
    pageSize,
    columns,
    lastViewedGameId,
    onSelectGame,
    onPageChange,
    gridRef
}: GameGridViewProps) {

    const { ref } = useFocusable({
        trackChildren: true
    });

    useEffect(() => {
        if (!isLoading && games.length > 0) {
            setTimeout(() => {
                if (lastViewedGameId && games.some(g => g.id === lastViewedGameId)) {
                    setFocus(`game-${lastViewedGameId}`);
                } else {
                    setFocus(`game-${games[0].id}`);
                }
            }, 100);
        }
    }, [games.length, isLoading, lastViewedGameId]);

    return (
        <div className="game-grid-view" ref={ref}>
            <div className="nav-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60px', position: 'relative' }}>
                <h1 style={{ margin: 0 }}>{platform.name}</h1>
                <span style={{ position: 'absolute', right: '40px', opacity: 0.6, fontSize: '0.9rem' }}>
                    {totalGames > 0 ? `${offset + 1}-${Math.min(offset + pageSize, totalGames)} of ${totalGames}` : '0 games'}
                </span>
            </div>

            <div className="grid-container" ref={gridRef}>
                {isLoading ? (
                    <div style={{ padding: '40px', textAlign: 'center', width: '100%', opacity: 0.6 }}>
                        Loading games...
                    </div>
                ) : games.length > 0 ? (
                    games.map((game, index) => (
                        <GameCard
                            key={game.id}
                            game={game}
                            isLeftmost={index % columns === 0}
                            isTopRow={index < columns}
                            onClick={() => onSelectGame(game)}
                        />
                    ))
                ) : (
                    <div style={{ padding: '40px', textAlign: 'center', width: '100%', opacity: 0.6 }}>
                        No games found for this platform.
                    </div>
                )}
            </div>

            {!isLoading && (offset > 0 || (offset + pageSize < totalGames)) && (
                <div className="pagination-controls" style={{ display: 'flex', justifyContent: 'center', gap: '20px', padding: '20px', paddingBottom: '80px', borderTop: '1px solid rgba(255,255,255,0.1)' }}>
                    {offset > 0 && (
                        <FocusableButton
                            focusKey="prev-page"
                            className="btn"
                            onEnterPress={() => onPageChange(offset - pageSize)}
                            onClick={() => onPageChange(offset - pageSize)}
                            onMouseEnter={() => {
                                if (getMouseActive()) setFocus('prev-page');
                            }}
                            style={{ padding: '8px 20px', minWidth: '120px' }}
                        >
                            Previous
                        </FocusableButton>
                    )}
                    {offset + pageSize < totalGames && (
                        <FocusableButton
                            focusKey="next-page"
                            className="btn"
                            onEnterPress={() => onPageChange(offset + pageSize)}
                            onClick={() => onPageChange(offset + pageSize)}
                            onMouseEnter={() => {
                                if (getMouseActive()) setFocus('next-page');
                            }}
                            style={{ padding: '8px 20px', minWidth: '120px' }}
                        >
                            Next
                        </FocusableButton>
                    )}
                </div>
            )}
        </div>
    );
}
