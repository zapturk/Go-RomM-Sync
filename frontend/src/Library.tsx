import { useState, useEffect } from 'react';
import { GetLibrary, GetPlatforms } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GameCard } from "./GameCard";
import { PlatformCard } from "./PlatformCard";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';

function Library() {
    const [games, setGames] = useState<types.Game[]>([]);
    const [platforms, setPlatforms] = useState<types.Platform[]>([]);
    const [status, setStatus] = useState("Loading library...");
    const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
    const [syncTrigger, setSyncTrigger] = useState(0);

    const { ref, focusKey } = useFocusable({
        trackChildren: true
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
            if ((e.code === 'Backspace' || e.code === 'Escape') && selectedPlatform) {
                e.preventDefault();
                setSelectedPlatform(null);
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [selectedPlatform]);

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
        if (selectedPlatform) {
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
    }, [selectedPlatform, games, platforms, setFocus]);

    return (
        <div id="library" ref={ref}>
            {!selectedPlatform ? (
                // Platform Grid View
                <>
                    <h1>Platforms</h1>
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
                    <div className="nav-header">
                        <h1>{selectedPlatform}</h1>
                    </div>
                    <div className="grid-container">
                        {selectedPlatform && platforms.find(p => p.name === selectedPlatform) ?
                            getGamesForPlatform(platforms.find(p => p.name === selectedPlatform)!).map((game) => (
                                <GameCard
                                    key={game.id}
                                    game={game}
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
                </div>
            </div>
        </div>
    );
}

export default Library;
