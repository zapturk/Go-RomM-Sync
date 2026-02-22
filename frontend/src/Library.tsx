import { useState, useEffect, useRef } from 'react';
import { GetLibrary, GetPlatforms, SelectRetroArchExecutable, Quit } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GameCard } from "./GameCard";
import { PlatformCard } from "./PlatformCard";
import { GamePage } from "./GamePage";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { SettingsIcon } from './components/Icons';

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
    const lastViewedGameId = useRef<number | null>(null);
    const lastViewedPlatformId = useRef<number | null>(null);
    const [syncTrigger, setSyncTrigger] = useState(0);
    const gridRef = useRef<HTMLDivElement>(null);
    const [columns, setColumns] = useState(1);

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
    }, [selectedPlatform]); // Re-run when view toggles to ensure new ref is observed

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    const { ref: configRef, focused: configFocused, focusSelf: focusConfig } = useFocusable({
        focusKey: 'config-button',
        onEnterPress: () => {
            onOpenSettings();
        },
        onArrowPress: (direction: string) => {
            if (direction === 'up' || direction === 'left') {
                return false;
            }
            return true;
        }
    });

    const refreshLibrary = () => {
        setStatus("Syncing...");
        setSyncTrigger(prev => prev + 1);

        // Fetch games
        GetLibrary()
            .then((result) => {
                console.log("Library fetched:", result);
                setGames(result);
            })
            .catch((err) => {
                console.error("Failed to fetch library:", err);
                setStatus("Error: " + err);
            });

        // Fetch platforms
        GetPlatforms()
            .then((result) => {
                console.log("Platforms fetched:", result);
                setPlatforms(result);
                setStatus("Ready");
            })
            .catch((err) => {
                console.error("Failed to fetch platforms:", err);
                setStatus("Error: " + err);
            });
    };

    useEffect(() => {
        refreshLibrary();
    }, []);

    // Handle "Back" navigation (Escape only)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.code === 'Escape') {
                if (selectedGameId) {
                    e.preventDefault();
                    setSelectedGameId(null);
                } else if (selectedPlatform) {
                    e.preventDefault();
                    setSelectedPlatform(null);
                }
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [selectedPlatform, selectedGameId]);

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
    }, [isActive]);

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

    // Helper to get games for a platform
    const getGamesForPlatform = (platform: types.Platform) => {
        return games.filter(game => {
            return game.full_path.includes("/" + platform.name + "/") ||
                game.full_path.includes("/" + platform.slug + "/");
        });
    };

    const visiblePlatforms = sortedPlatforms.filter(p => getGamesForPlatform(p).length > 0);

    // Auto-focus first platform or last viewed platform when configured
    useEffect(() => {
        if (!isActive) return;

        if (!selectedPlatform && visiblePlatforms.length > 0) {
            // Give a moment for the DOM to settle
            setTimeout(() => {
                if (lastViewedPlatformId.current && visiblePlatforms.some(p => p.id === lastViewedPlatformId.current)) {
                    setFocus(`platform-${lastViewedPlatformId.current}`);
                    lastViewedPlatformId.current = null; // Clear after restoring
                } else {
                    const key = `platform-${visiblePlatforms[0].id}`;
                    setFocus(key);
                }
            }, 100);
        } else if (!selectedPlatform && visiblePlatforms.length === 0) {
            // If no platforms, focus the settings button so the user can fix the config
            setTimeout(() => {
                focusConfig();
            }, 100);
        }
    }, [visiblePlatforms.length, selectedPlatform, setFocus, isActive]);

    // Ensure focus is restored when coming back from settings
    useEffect(() => {
        if (isActive) {
            // Small delay to ensure the view is visible
            setTimeout(() => {
                // If we're on the main platform screen, the other effect might handle it,
                // but if we're deeper in the UI or nothing is focused, this ensures survival.
                if (!document.querySelector('.focused')) {
                    focusConfig();
                }
            }, 50);
        }
    }, [isActive, focusConfig]);

    // Auto-focus first game or last viewed game when platform selected
    useEffect(() => {
        if (!isActive) return;

        if (selectedPlatform && !selectedGameId) {
            const platform = platforms.find(p => p.name === selectedPlatform);
            if (platform) {
                const platformGames = getGamesForPlatform(platform);
                if (platformGames.length > 0) {
                    setTimeout(() => {
                        // Check if we have a last viewed game to restore focus to
                        if (lastViewedGameId.current && platformGames.some(g => g.id === lastViewedGameId.current)) {
                            setFocus(`game-${lastViewedGameId.current}`);
                            lastViewedGameId.current = null; // Clear after restoring
                        } else {
                            // Default to first game
                            const key = `game-${platformGames[0].id}`;
                            setFocus(key);
                        }
                    }, 100);
                }
            }
        }
    }, [selectedPlatform, selectedGameId, games, platforms, setFocus, isActive]);

    if (selectedGameId) {
        return (
            <GamePage
                gameId={selectedGameId}
                onBack={() => setSelectedGameId(null)}
            />
        );
    }

    return (
        <div id="library" ref={ref}>
            {!selectedPlatform ? (
                // Platform Grid View
                <>
                    <div className="nav-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', position: 'relative', minHeight: '60px' }}>
                        <button
                            ref={configRef}
                            className={`btn config-btn ${configFocused ? 'focused' : ''}`}
                            title="Open Settings"
                            style={{
                                margin: 0,
                                padding: '5px',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center',
                                background: 'transparent',
                                border: '1px solid rgba(255,255,255,0.1)',
                                position: 'absolute',
                                left: '40px'
                            }}
                            onMouseEnter={() => {
                                if (getMouseActive()) {
                                    focusConfig();
                                }
                            }}
                            onClick={onOpenSettings}
                        >
                            <SettingsIcon size={24} />
                        </button>
                        <h1 style={{ margin: 0 }}>Platforms</h1>
                    </div>
                    <div className="grid-container" ref={gridRef}>
                        {visiblePlatforms.map((platform, index) => (
                            <PlatformCard
                                key={platform.id}
                                platform={platform}
                                isLeftmost={index % columns === 0}
                                onClick={() => {
                                    setSelectedPlatform(platform.name);
                                    lastViewedPlatformId.current = platform.id;
                                }}
                                onEnterPress={() => {
                                    setSelectedPlatform(platform.name);
                                    lastViewedPlatformId.current = platform.id;
                                }}
                                syncTrigger={syncTrigger}
                            />
                        ))}
                    </div>
                </>
            ) : (
                // Game Grid View
                <>
                    <div className="nav-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60px' }}>
                        <h1 style={{ margin: 0 }}>{selectedPlatform}</h1>
                    </div>
                    <div className="grid-container" ref={gridRef}>
                        {selectedPlatform && platforms.find(p => p.name === selectedPlatform) ?
                            getGamesForPlatform(platforms.find(p => p.name === selectedPlatform)!).map((game, index) => (
                                <GameCard
                                    key={game.id}
                                    game={game}
                                    isLeftmost={index % columns === 0}
                                    isTopRow={index < columns}
                                    onClick={() => {
                                        setSelectedGameId(game.id);
                                        lastViewedGameId.current = game.id;
                                    }}
                                />
                            ))
                            : <p>No games found (mapping issue?)</p>
                        }
                    </div>
                </>
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
