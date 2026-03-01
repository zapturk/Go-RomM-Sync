import { useRef, useEffect } from 'react';
import { types } from "../../../wailsjs/go/models";
import { GameCard } from "../../GameCard";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';

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

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    const { ref: prevRef, focused: prevFocused, focusSelf: focusPrev } = useFocusable({
        focusKey: 'prev-page',
        onEnterPress: () => {
            if (offset > 0) onPageChange(offset - pageSize);
        }
    });

    const { ref: nextRef, focused: nextFocused, focusSelf: focusNext } = useFocusable({
        focusKey: 'next-page',
        onEnterPress: () => {
            if (offset + pageSize < totalGames) onPageChange(offset + pageSize);
        }
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

            <div className="pagination-controls" style={{ display: 'flex', justifyContent: 'center', gap: '20px', padding: '20px', paddingBottom: '80px', borderTop: '1px solid rgba(255,255,255,0.1)' }}>
                {offset > 0 && (
                    <button
                        ref={prevRef}
                        className={`btn ${prevFocused ? 'focused' : ''}`}
                        onClick={() => onPageChange(offset - pageSize)}
                        style={{ padding: '8px 20px', minWidth: '120px' }}
                    >
                        Previous
                    </button>
                )}
                {offset + pageSize < totalGames && (
                    <button
                        ref={nextRef}
                        className={`btn ${nextFocused ? 'focused' : ''}`}
                        onClick={() => onPageChange(offset + pageSize)}
                        style={{ padding: '8px 20px', minWidth: '120px' }}
                    >
                        Next
                    </button>
                )}
            </div>
        </div>
    );
}
