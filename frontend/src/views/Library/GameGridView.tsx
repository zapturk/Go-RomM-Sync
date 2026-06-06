import React, { useRef, useEffect, useState } from 'react';
import { types } from "../../../wailsjs/go/models";
import { GameCard } from "../../GameCard";
import { useFocusable, setFocus, getCurrentFocusKey } from '@noriginmedia/norigin-spatial-navigation';
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
    gridRef: React.RefObject<HTMLDivElement | null>;
    isActive: boolean;
}

const shouldSkipGameFocus = (currentFocus: string | null): boolean => {
    if (!currentFocus) return false;
    return currentFocus.startsWith('game-') || currentFocus === 'prev-page' || currentFocus === 'next-page';
};

const getTargetGameId = (games: types.Game[], lastViewedGameId: number | null): number => {
    if (lastViewedGameId && games.some(g => g.id === lastViewedGameId)) {
        return lastViewedGameId;
    }
    return games[0].id;
};

const handleSearchInputKeyDown = (
    e: React.KeyboardEvent<HTMLInputElement>,
    games: types.Game[],
    searchInputRef: React.RefObject<HTMLInputElement | null>
) => {
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
};

interface GamePaginationProps {
    isLoading: boolean;
    offset: number;
    pageSize: number;
    totalGames: number;
    onPageChange: (newOffset: number) => void;
}

function GamePagination({ isLoading, offset, pageSize, totalGames, onPageChange }: GamePaginationProps) {
    if (isLoading) return null;
    const hasPrev = offset > 0;
    const hasNext = offset + pageSize < totalGames;
    if (!hasPrev && !hasNext) return null;

    return (
        <div className="pagination-controls">
            {hasPrev && (
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
            {hasNext && (
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
    );
}

interface GameGridContentProps {
    isLoading: boolean;
    games: types.Game[];
    columns: number;
    onSelectGame: (game: types.Game) => void;
}

function GameGridContent({ isLoading, games, columns, onSelectGame }: GameGridContentProps) {
    if (isLoading) {
        return (
            <div style={{ padding: '40px', textAlign: 'center', width: '100%', opacity: 0.6 }}>
                Loading games...
            </div>
        );
    }
    if (games.length === 0) {
        return (
            <div style={{ padding: '40px', textAlign: 'center', width: '100%', opacity: 0.6 }}>
                No games found for this platform.
            </div>
        );
    }
    return (
        <>
            {games.map((game, index) => (
                <GameCard
                    key={game.id}
                    game={game}
                    isLeftmost={index % columns === 0}
                    isTopRow={index < columns}
                    onClick={() => onSelectGame(game)}
                />
            ))}
        </>
    );
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
        if (!isActive || isLoading || games.length === 0) return;

        const timerId = setTimeout(() => {
            const currentFocus = getCurrentFocusKey();
            if (shouldSkipGameFocus(currentFocus)) return;
            if (document.activeElement === searchInputRef.current) return;

            setFocus(`game-${getTargetGameId(games, lastViewedGameId)}`);
        }, 100);

        return () => clearTimeout(timerId);
    }, [games, isLoading, lastViewedGameId, isActive]);

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
                        onKeyDown={(e) => handleSearchInputKeyDown(e, games, searchInputRef)}
                    />
                </div>

                <span className="pagination-info">
                    {totalGames > 0 ? `${offset + 1}-${Math.min(offset + pageSize, totalGames)} of ${totalGames}` : '0 games'}
                </span>
            </div>

            <div className="grid-container" ref={gridRef}>
                <GameGridContent
                    isLoading={isLoading}
                    games={games}
                    columns={columns}
                    onSelectGame={onSelectGame}
                />
            </div>

            <GamePagination
                isLoading={isLoading}
                offset={offset}
                pageSize={pageSize}
                totalGames={totalGames}
                onPageChange={onPageChange}
            />
        </div>
    );
}
