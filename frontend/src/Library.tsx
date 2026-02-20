import { useState, useEffect } from 'react';
import { GetLibrary, GetPlatforms, SelectRetroArchExecutable, Quit } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GameCard } from "./GameCard";
import { PlatformCard } from "./PlatformCard";
import { GamePage } from "./GamePage";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';

const SettingsIcon = ({ size = 24 }: { size?: number }) => (
    <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
    >
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
);
interface LibraryProps {
    onOpenSettings: () => void;
}

function Library({ onOpenSettings }: LibraryProps) {
    const [games, setGames] = useState<types.Game[]>([]);
    const [platforms, setPlatforms] = useState<types.Platform[]>([]);
    const [status, setStatus] = useState("Loading library...");
    const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
    const [selectedGameId, setSelectedGameId] = useState<number | null>(null);
    const [syncTrigger, setSyncTrigger] = useState(0);

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    const { ref: configRef, focused: configFocused, focusSelf: focusConfig } = useFocusable({
        focusKey: 'config-button',
        onEnterPress: () => {
            onOpenSettings();
        },
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

    // Handle "Back" navigation (Backspace/Escape)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.code === 'Backspace' || e.code === 'Escape') {
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
            if (e.key === 'r' || e.key === 'R') {
                e.preventDefault();
                refreshLibrary();
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, []);

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

    // Auto-focus first platform when configured
    useEffect(() => {
        if (!selectedPlatform && visiblePlatforms.length > 0) {
            // Give a moment for the DOM to settle
            setTimeout(() => {
                const key = `platform-${visiblePlatforms[0].id}`;
                setFocus(key);
            }, 100);
        }
    }, [visiblePlatforms.length, selectedPlatform, setFocus]);

    // Auto-focus first game when platform selected
    useEffect(() => {
        if (selectedPlatform && !selectedGameId) {
            const platform = platforms.find(p => p.name === selectedPlatform);
            if (platform) {
                const platformGames = getGamesForPlatform(platform);
                if (platformGames.length > 0) {
                    setTimeout(() => {
                        const key = `game-${platformGames[0].id}`;
                        setFocus(key);
                    }, 100);
                }
            }
        }
    }, [selectedPlatform, selectedGameId, games, platforms, setFocus]);

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
                    <div className="grid-container">
                        {visiblePlatforms.map((platform) => (
                            <PlatformCard
                                key={platform.id}
                                platform={platform}
                                onClick={() => setSelectedPlatform(platform.name)}
                                onEnterPress={() => {
                                    setSelectedPlatform(platform.name);
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
                    <div className="grid-container">
                        {selectedPlatform && platforms.find(p => p.name === selectedPlatform) ?
                            getGamesForPlatform(platforms.find(p => p.name === selectedPlatform)!).map((game) => (
                                <GameCard
                                    key={game.id}
                                    game={game}
                                    onClick={() => setSelectedGameId(game.id)}
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
