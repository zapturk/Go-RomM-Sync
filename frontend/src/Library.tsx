import { useState, useEffect, useRef } from 'react';
import { GetLibrary, GetPlatforms, Quit, GetConfig, LogInfof } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { GamePage } from "./GamePage";
import { PlatformGridView } from "./views/Library/PlatformGridView";
import { GameGridView } from "./views/Library/GameGridView";
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';
import { LegendItem } from './components/LegendItem';
interface LibraryProps {
    onOpenSettings: () => void;
    isActive?: boolean;
}
const isTypingActive = (): boolean => {
    const el = document.activeElement;
    return el?.tagName === 'INPUT' || el?.tagName === 'TEXTAREA';
};


const handleLibraryNavigation = (
    e: KeyboardEvent,
    selectedGameId: number | null,
    selectedPlatform: string | null,
    offset: number,
    platformOffset: number,
    totalGames: number,
    totalPlatforms: number,
    pageSize: number,
    setSelectedGameId: (id: number | null) => void,
    setSelectedPlatform: (p: string | null) => void,
    handlePageChange: (offset: number) => void,
    handlePlatformPageChange: (offset: number) => void
) => {
    if (selectedGameId) {
        if (e.key === 'Escape') {
            e.preventDefault();
            setSelectedGameId(null);
        }
        return;
    }

    if (e.key === 'Escape') {
        e.preventDefault();
        if (selectedPlatform) setSelectedPlatform(null);
        return;
    }

    if (e.key === 'PageUp') {
        e.preventDefault();
        if (selectedPlatform && offset > 0) {
            handlePageChange(offset - pageSize);
        } else if (!selectedPlatform && platformOffset > 0) {
            handlePlatformPageChange(platformOffset - pageSize);
        }
        return;
    }

    if (e.key === 'PageDown') {
        e.preventDefault();
        if (selectedPlatform && offset + pageSize < totalGames) {
            handlePageChange(offset + pageSize);
        } else if (!selectedPlatform && platformOffset + pageSize < totalPlatforms) {
            handlePlatformPageChange(platformOffset + pageSize);
        }
        return;
    }
};

function Library({ onOpenSettings, isActive = true }: LibraryProps) {
    const [games, setGames] = useState<types.Game[]>([]);
    const [platforms, setPlatforms] = useState<types.Platform[]>([]);
    const [status, setStatus] = useState("Loading library...");
    const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
    const [selectedGameId, setSelectedGameId] = useState<number | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const lastViewedGameId = useRef<number | null>(null);
    const lastViewedPlatformId = useRef<number | null>(null);
    const [syncTrigger, setSyncTrigger] = useState(0);
    const gridRef = useRef<HTMLDivElement>(null);
    const columns = 6;
    const [searchTerm, setSearchTerm] = useState("");
    const [offlineMode, setOfflineMode] = useState(false);
    const requestCounter = useRef(0);

    // Pagination state
    const [offset, setOffset] = useState(0);
    const [totalGames, setTotalGames] = useState(0);
    const [platformOffset, setPlatformOffset] = useState(0);
    const [totalPlatforms, setTotalPlatforms] = useState(0);
    const PAGE_SIZE = 30;

    const selectPlatform = (platformName: string | null) => {
        LogInfof("selectPlatform: platformName=%s", [platformName || "null"]);
        setGames([]);
        setOffset(0);
        setTotalGames(0);
        setIsLoading(true);
        setSelectedPlatform(platformName);
    };

    const handleSearch = (term: string) => {
        setGames([]);
        setOffset(0);
        setTotalGames(0);
        setIsLoading(true);
        setSearchTerm(term);
    };

    useEffect(() => {
        if (!selectedPlatform) {
            setSearchTerm("");
        }
    }, [selectedPlatform]);

    const { ref } = useFocusable({
        trackChildren: true
    });

    const refreshLibrary = (currentOffset: number = offset, currentPlatformOffset: number = platformOffset, currentSearch: string = searchTerm) => {
        setIsLoading(true);
        setStatus("Syncing...");
        setSyncTrigger(prev => prev + 1);

        requestCounter.current += 1;
        const myRequestId = requestCounter.current;

        GetConfig().then(cfg => {
            if (myRequestId !== requestCounter.current) return;
            setOfflineMode(cfg.offline_mode || false);
        });

        LogInfof("refreshLibrary: selectedPlatform=%s, platformsCount=%d, currentOffset=%d, currentSearch=%s", [selectedPlatform || "null", platforms.length, currentOffset, currentSearch]);
        const activePlatform = platforms.find(p => p.name === selectedPlatform);
        const platformId = activePlatform?.id || 0;
        LogInfof("refreshLibrary resolved: platformId=%d (activePlatform=%s)", [platformId, activePlatform ? activePlatform.name : "null"]);

        const gamesPromise = GetLibrary(PAGE_SIZE, currentOffset, platformId, currentSearch)
            .then((result) => {
                if (myRequestId !== requestCounter.current) return;
                setGames(result.items || []);
                setTotalGames(result.total || 0);
            })
            .catch((err) => {
                if (myRequestId !== requestCounter.current) return;
                console.error("Failed to fetch library:", err);
                setStatus("Error: " + err);
            });

        const platformsPromise = GetPlatforms(PAGE_SIZE, currentPlatformOffset)
            .then((result) => {
                if (myRequestId !== requestCounter.current) return;
                setPlatforms(result.items || []);
                setTotalPlatforms(result.total || 0);
                setStatus("Ready");
            })
            .catch((err) => {
                if (myRequestId !== requestCounter.current) return;
                console.error("Failed to fetch platforms:", err);
                setStatus("Error: " + err);
            });

        Promise.allSettled([gamesPromise, platformsPromise]).finally(() => {
            if (myRequestId !== requestCounter.current) return;
            setIsLoading(false);
        });
    };

    const handlePageChange = (newOffset: number) => {
        setOffset(newOffset);
        refreshLibrary(newOffset, platformOffset);
        if (gridRef.current) {
            gridRef.current.scrollTop = 0;
        }
    };

    const handlePlatformPageChange = (newOffset: number) => {
        setPlatformOffset(newOffset);
        refreshLibrary(offset, newOffset);
        if (gridRef.current) {
            gridRef.current.scrollTop = 0;
        }
    };

    useEffect(() => {
        // Clear games when platform changes to prevent flicker
        setGames([]);
        setOffset(0);
        setTotalGames(0);
        refreshLibrary(0, platformOffset, searchTerm);
    }, [selectedPlatform, searchTerm]);

    // Handle "Back" navigation (Escape/B Button) and Pagination (PageUp/PageDown / LB/RB)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (!isActive) return;
            handleLibraryNavigation(
                e,
                selectedGameId,
                selectedPlatform,
                offset,
                platformOffset,
                totalGames,
                totalPlatforms,
                PAGE_SIZE,
                setSelectedGameId,
                selectPlatform,
                handlePageChange,
                handlePlatformPageChange
            );
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [selectedPlatform, selectedGameId, isActive, offset, platformOffset, totalGames, totalPlatforms]);

    // Handle offline-mode-changed event
    useEffect(() => {
        const unsubscribe = EventsOn("offline-mode-changed", (newOfflineMode: boolean) => {
            setOfflineMode(newOfflineMode);
            if (newOfflineMode) {
                // If we switched to offline, refresh to filter only local games
                setGames([]);
                setOffset(0);
                setTotalGames(0);
                setIsLoading(true);
                refreshLibrary(0, platformOffset, searchTerm);
            }
        });
        return () => unsubscribe();
    }, [platformOffset, searchTerm]);

    // Handle "Refresh" (R key)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (!isActive) return;
            if (isTypingActive()) return;

            if (e.key.toLowerCase() === 'r') {
                e.preventDefault();
                refreshLibrary();
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [isActive, offset, selectedPlatform, platforms]);

    // Handle "Exit" (Alt+F4 or similar)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            const isMac = navigator.userAgent.includes('Mac');
            // Check for Alt+F4 (Windows/Linux) or Cmd+Q (Mac)
            if ((e.altKey && e.key === 'F4') || (isMac && e.metaKey && e.key === 'q')) {
                e.preventDefault();
                Quit();
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, []);

    const sortedPlatforms = [...platforms].sort((a, b) => a.name.localeCompare(b.name));


    const currentPlatformObj = platforms.find(p => p.name === selectedPlatform);

    if (selectedGameId) {
        return (
            <GamePage
                gameId={selectedGameId}
                onBack={() => setSelectedGameId(null)}
            />
        );
    }

    return (
        <div id="library-root" ref={ref} style={{ height: '100%' }}>
            <div id="library">
                <div className="library-header-extras" style={{ top: '2rem', right: '8rem' }}>
                    {offlineMode && (
                        <div className="offline-badge">
                            Offline Mode
                        </div>
                    )}
                </div>
                {!selectedPlatform || !currentPlatformObj ? (
                    <PlatformGridView
                        platforms={sortedPlatforms}
                        isLoading={isLoading}
                        isActive={isActive}
                        offset={platformOffset}
                        totalPlatforms={totalPlatforms}
                        pageSize={PAGE_SIZE}
                        onSelectPlatform={(p) => {
                            selectPlatform(p.name);
                            lastViewedPlatformId.current = p.id;
                        }}
                        onPageChange={handlePlatformPageChange}
                        onOpenSettings={onOpenSettings}
                        columns={columns}
                        syncTrigger={syncTrigger}
                        lastViewedPlatformId={lastViewedPlatformId.current}
                        gridRef={gridRef}
                    />
                ) : (
                    <GameGridView
                        platform={currentPlatformObj}
                        games={games}
                        isLoading={isLoading}
                        isActive={isActive}
                        offset={offset}
                        totalGames={totalGames}
                        pageSize={PAGE_SIZE}
                        columns={columns}
                        lastViewedGameId={lastViewedGameId.current}
                        onSelectGame={(g) => {
                            setSelectedGameId(g.id);
                            lastViewedGameId.current = g.id;
                        }}
                        onPageChange={handlePageChange}
                        searchTerm={searchTerm}
                        onSearchChange={handleSearch}
                        gridRef={gridRef}
                    />
                )}

                <div className="input-legend">
                    <div className="footer-left">
                        <span>{status}</span>
                    </div>
                    <div className="footer-right">
                        <div className="legend-item">
                            <div className="show-gamepad">
                                <div className="icon-group">
                                    <div className="pill-icon">LB</div>
                                    <div className="pill-icon">RB</div>
                                </div>
                            </div>
                            <div className="show-keyboard">
                                <div className="icon-group">
                                    <div className="key-icon" style={{ width: 'auto', padding: '0 4px' }}>PgUp</div>
                                    <div className="key-icon" style={{ width: 'auto', padding: '0 4px' }}>PgDn</div>
                                </div>
                            </div>
                            <span>Page</span>
                        </div>

                        <LegendItem buttonAction="west" keyLabel="R" label="Sync" />

                        {selectedPlatform && (
                            <LegendItem buttonAction="east" keyLabel="ESC" label="Back" />
                        )}

                        <LegendItem buttonAction="south" keyLabel="ENTER" label="OK" />

                        <div className="legend-item">
                            <div className="show-gamepad">
                                <div className="icon-group">
                                    <div className="pill-icon">SELECT</div>
                                    <div className="pill-icon">START</div>
                                </div>
                            </div>
                            <div className="key-icon show-keyboard" style={{ width: 'auto', padding: '0 4px' }}>
                                {navigator.userAgent.includes('Mac') ? 'CMD + Q' : 'ALT + F4'}
                            </div>
                            <span>Exit</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default Library;
