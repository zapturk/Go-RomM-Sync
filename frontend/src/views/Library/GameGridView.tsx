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
    isActive: boolean;
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
    gridRef,
    isActive
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
    }, [localSearch]);

    useEffect(() => {
        if (!isActive) return;

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
    }, [games.length, isLoading, lastViewedGameId, isActive]);

    return (
        <div className="game-grid-view" ref={ref}>
            <div className="nav-header">
                <h1>{platform.name}</h1>

                <div className="search-container">
                    <input
                        ref={searchInputRef}
                        type="text"
                        className="search-input"
                        placeholder="Search games..."
                        value={localSearch}
                        onChange={(e) => setLocalSearch(e.target.value)}
                        onKeyDown={(e) => {
                            switch (e.key) {
                                case 'Enter':
                                    e.stopPropagation();
                                    break;
                                case 'Escape':
                                    searchInputRef.current?.blur();
                                    if (games.length > 0) {
                                        setFocus(`game-${games[0].id}`);
                                    }
                                    e.preventDefault();
                                    e.stopPropagation();
                                    break;
                                case 'ArrowDown':
                                    if (games.length > 0) {
                                        setFocus(`game-${games[0].id}`);
                                        e.preventDefault();
                                    }
                                    break;
                            }
                        }}
                    />
                </div>

                <span className="pagination-info">
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
                <div className="pagination-controls">
                    {offset > 0 && (
                        <FocusableButton
                            focusKey="prev-page"
                            className="btn pagination-btn"
                            onEnterPress={() => onPageChange(offset - pageSize)}
                            onClick={() => onPageChange(offset - pageSize)}
                            onMouseEnter={() => {
                                if (getMouseActive()) setFocus('prev-page');
                            }}
                        >
                            Previous
                        </FocusableButton>
                    )}
                    {offset + pageSize < totalGames && (
                        <FocusableButton
                            focusKey="next-page"
                            className="btn pagination-btn"
                            onEnterPress={() => onPageChange(offset + pageSize)}
                            onClick={() => onPageChange(offset + pageSize)}
                            onMouseEnter={() => {
                                if (getMouseActive()) setFocus('next-page');
                            }}
                        >
                            Next
                        </FocusableButton>
                    )}
                </div>
            )}
        </div>
    );
}
