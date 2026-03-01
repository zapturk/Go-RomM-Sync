import { useState, useEffect, useRef } from 'react';
import { GetLibrary, GetPlatforms, Quit } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GamePage } from "./GamePage";
import { PlatformGridView } from "./views/Library/PlatformGridView";
import { GameGridView } from "./views/Library/GameGridView";
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';

interface LibraryProps {
    onOpenSettings: () => void;
    isActive?: boolean;
}

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
    const [columns, setColumns] = useState(1);

    // Pagination state
    const [offset, setOffset] = useState(0);
    const [totalGames, setTotalGames] = useState(0);
    const [platformOffset, setPlatformOffset] = useState(0);
    const [totalPlatforms, setTotalPlatforms] = useState(0);
    const PAGE_SIZE = 25;

    useEffect(() => {
        const updateColumns = () => {
            if (gridRef.current) {
                const containerWidth = gridRef.current.offsetWidth;
                const itemWidth = 200; // min-width from App.css
                const gap = 20; // gap from App.css
                const cols = Math.floor((containerWidth + gap) / (itemWidth + gap));
                setColumns(cols || 1);
            }
        };

        const observer = new ResizeObserver(updateColumns);
        if (gridRef.current) {
            observer.observe(gridRef.current);
        }
        updateColumns();

        return () => observer.disconnect();
    }, [selectedPlatform]);

    const { ref } = useFocusable({
        trackChildren: true
    });

    const refreshLibrary = (currentOffset: number = offset, currentPlatformOffset: number = platformOffset) => {
        setIsLoading(true);
        setStatus("Syncing...");
        setSyncTrigger(prev => prev + 1);

        const activePlatform = platforms.find(p => p.name === selectedPlatform);
        const platformId = activePlatform?.id || 0;

        const gamesPromise = GetLibrary(PAGE_SIZE, currentOffset, platformId)
            .then((result) => {
                setGames(result.items || []);
                setTotalGames(result.total || 0);
            })
            .catch((err) => {
                console.error("Failed to fetch library:", err);
                setStatus("Error: " + err);
            });

        const platformsPromise = GetPlatforms(PAGE_SIZE, currentPlatformOffset)
            .then((result) => {
                setPlatforms(result.items || []);
                setTotalPlatforms(result.total || 0);
                setStatus("Ready");
            })
            .catch((err) => {
                console.error("Failed to fetch platforms:", err);
                setStatus("Error: " + err);
            });

        Promise.allSettled([gamesPromise, platformsPromise]).finally(() => {
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
        refreshLibrary(0);
    }, [selectedPlatform]);

    // Handle "Back" navigation (Escape/B Button)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (!isActive) return;

            if (e.code === 'Escape' || e.key === 'Escape') {
                e.preventDefault();

                if (selectedGameId) {
                    setSelectedGameId(null);
                } else if (selectedPlatform) {
                    setSelectedPlatform(null);
                }
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [selectedPlatform, selectedGameId, isActive]);

    // Handle "Refresh" (R key)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (!isActive) return;

            // Don't trigger sync if we're typing in an input
            const activeElement = document.activeElement;
            const isTyping = activeElement?.tagName === 'INPUT' || activeElement?.tagName === 'TEXTAREA';

            if (!isTyping && (e.key === 'r' || e.key === 'R')) {
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


    if (selectedGameId) {
        return (
            <GamePage
                gameId={selectedGameId}
                onBack={() => setSelectedGameId(null)}
            />
        );
    }

    const currentPlatformObj = platforms.find(p => p.name === selectedPlatform);

    return (
        <div id="library" ref={ref}>
            {!selectedPlatform || !currentPlatformObj ? (
                <PlatformGridView
                    platforms={sortedPlatforms}
                    isLoading={isLoading}
                    offset={platformOffset}
                    totalPlatforms={totalPlatforms}
                    pageSize={PAGE_SIZE}
                    onSelectPlatform={(p) => {
                        setSelectedPlatform(p.name);
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
                    gridRef={gridRef}
                />
            )}

            <div className="input-legend">
                <div className="footer-left">
                    <span>{status}</span>
                </div>
                <div className="footer-right">
                    <div className="legend-item">
                        {/* Gamepad Icon */}
                        <div className="btn-icon show-gamepad">
                            <div className="btn-dot north"></div>
                            <div className="btn-dot east"></div>
                            <div className="btn-dot south"></div>
                            <div className="btn-dot west active"></div>
                        </div>
                        {/* Keyboard Icon */}
                        <div className="key-icon show-keyboard">R</div>
                        <span>Sync</span>
                    </div>

                    {selectedPlatform && (
                        <div className="legend-item">
                            <div className="btn-icon show-gamepad">
                                <div className="btn-dot north"></div>
                                <div className="btn-dot east active"></div>
                                <div className="btn-dot south"></div>
                                <div className="btn-dot west"></div>
                            </div>
                            <div className="key-icon show-keyboard">ESC</div>
                            <span>Back</span>
                        </div>
                    )}

                    <div className="legend-item">
                        <div className="btn-icon show-gamepad">
                            <div className="btn-dot north"></div>
                            <div className="btn-dot east"></div>
                            <div className="btn-dot south active"></div>
                            <div className="btn-dot west"></div>
                        </div>
                        <div className="key-icon show-keyboard">ENTER</div>
                        <span>OK</span>
                    </div>

                    <div className="legend-item">
                        <div className="show-gamepad">
                            <div style={{ display: 'flex', gap: '4px' }}>
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
    );
}

export default Library;
