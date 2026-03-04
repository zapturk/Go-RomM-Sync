import { useRef, useEffect, useState } from 'react';
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
    searchTerm: string;
    onSearchChange: (value: string) => void;
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
    searchTerm,
    onSearchChange,
    gridRef
}: GameGridViewProps) {
    const [localSearch, setLocalSearch] = useState(searchTerm);
    const searchInputRef = useRef<HTMLInputElement>(null);

    const { ref } = useFocusable({
        trackChildren: true,
    });

    useEffect(() => {
        setLocalSearch(searchTerm);
    }, [searchTerm]);

    useEffect(() => {
        const handler = setTimeout(() => {
            if (localSearch !== searchTerm) {
                onSearchChange(localSearch);
            }
        }, 300);

        return () => clearTimeout(handler);
    }, [localSearch, onSearchChange, searchTerm]);

    useEffect(() => {
        if (!isLoading && games.length > 0) {
            setTimeout(() => {
                if (lastViewedGameId && games.some(g => g.id === lastViewedGameId)) {
                    setFocus(`game-${lastViewedGameId}`);
                } else {
                    // Don't auto-focus game list if we are already searching
                    if (document.activeElement === searchInputRef.current) {
                        return;
                    }
                    setFocus(`game-${games[0].id}`);
                }
            }, 100);
        }
    }, [games.length, isLoading, lastViewedGameId]);

    return (
        <div className="game-grid-view" ref={ref}>
            <div className="nav-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60px', position: 'relative', paddingRight: '200px', paddingLeft: '200px' }}>
                <h1 style={{ margin: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{platform.name}</h1>

                <div className="search-container" style={{ marginLeft: '40px', flex: 1, maxWidth: '400px' }}>
                    <input
                        ref={searchInputRef}
                        type="text"
                        className="search-input"
                        placeholder="Search games..."
                        value={localSearch}
                        onChange={(e) => setLocalSearch(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.stopPropagation();
                            }
                            if (e.key === 'Escape') {
                                searchInputRef.current?.blur();
                                if (games.length > 0) {
                                    setFocus(`game-${games[0].id}`);
                                }
                                e.preventDefault();
                                e.stopPropagation();
                            }
                            if (e.key === 'ArrowDown') {
                                if (games.length > 0) {
                                    setFocus(`game-${games[0].id}`);
                                    e.preventDefault();
                                }
                            }
                        }}
                        style={{
                            width: '100%',
                            padding: '8px 16px',
                            background: 'rgba(255, 255, 255, 0.05)',
                            border: '1px solid rgba(255, 255, 255, 0.1)',
                            borderRadius: '20px',
                            color: 'white',
                            fontSize: '0.9rem',
                            outline: 'none',
                            transition: 'all 0.2s'
                        }}
                    />
                </div>

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
