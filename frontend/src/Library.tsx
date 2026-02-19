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

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    const refreshLibrary = () => {
        setStatus("Refreshing library...");

        // Fetch games
        GetLibrary()
            .then((result) => {
                console.log("Library fetched:", result);
                setGames(result);
            })
            .catch((err) => {
                console.error("Failed to fetch library:", err);
                setStatus("Error loading library: " + err);
            });

        // Fetch platforms
        GetPlatforms()
            .then((result) => {
                console.log("Platforms fetched:", result);
                setPlatforms(result);
                setStatus("Library and Platforms loaded!");
            })
            .catch((err) => {
                console.error("Failed to fetch platforms:", err);
                setStatus("Error loading platforms: " + err);
            });
    };

    useEffect(() => {
        refreshLibrary();
    }, []);

    // Handle "Back" navigation (Backspace/Escape)
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if ((e.code === 'Backspace' || e.code === 'Escape') && selectedPlatform) {
                setSelectedPlatform(null);
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [selectedPlatform]);

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
                    <button className="btn" onClick={refreshLibrary} style={{ marginBottom: "1rem" }}>Sync Library</button>
                    <p>{status}</p>
                    <div className="grid-container">
                        {visiblePlatforms.map((platform) => (
                            <PlatformCard
                                key={platform.id}
                                platform={platform}
                                onClick={() => setSelectedPlatform(platform.name)}
                                onEnterPress={() => {
                                    setSelectedPlatform(platform.name);
                                }}
                            />
                        ))}
                    </div>
                </>
            ) : (
                // Game Grid View
                <>
                    <div className="nav-header">
                        <button className="back-btn" onClick={() => setSelectedPlatform(null)}>
                            ← Back
                        </button>
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
        </div>
    );
}

export default Library;
