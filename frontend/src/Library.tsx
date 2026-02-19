import { useState, useEffect } from 'react';
import { GetLibrary } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";

function Library() {
    const [games, setGames] = useState<types.Game[]>([]);
    const [status, setStatus] = useState("Loading library...");

    const refreshLibrary = () => {
        setStatus("Refreshing library...");
        GetLibrary()
            .then((result) => {
                console.log("Library fetched:", result);
                setGames(result);
                setStatus("Library loaded!");
            })
            .catch((err) => {
                console.error("Failed to fetch library:", err);
                setStatus("Error loading library: " + err);
            });
    };

    useEffect(() => {
        refreshLibrary();
    }, []);

    const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);

    // Extract unique platforms from game paths
    // Assumes path format like ".../Console Name/Game.ext" or similar
    const getPlatformFromPath = (path: string): string => {
        if (!path) return "Unknown";
        const parts = path.split('/');
        // Assuming the folder containing the file is the platform
        // e.g. /library/roms/NES/mario.nes -> NES
        // If path ends with file, take distinct parent
        if (parts.length >= 2) {
            return parts[parts.length - 2];
        }
        return "Unknown";
    };

    // Group games by platform
    const gamesByPlatform = games.reduce((acc, game) => {
        const platform = getPlatformFromPath(game.full_path);
        if (!acc[platform]) {
            acc[platform] = [];
        }
        acc[platform].push(game);
        return acc;
    }, {} as Record<string, types.Game[]>);

    const platforms = Object.keys(gamesByPlatform).sort();

    return (
        <div id="library">
            {!selectedPlatform ? (
                // Platform Grid View
                <>
                    <h1>Game Configurations</h1>
                    <button className="btn" onClick={refreshLibrary} style={{ marginBottom: "1rem" }}>Sync Library</button>
                    <p>{status}</p>
                    <div className="grid-container">
                        {platforms.map((platform) => (
                            <div
                                key={platform}
                                className="card"
                                onClick={() => setSelectedPlatform(platform)}
                            >
                                <h3>{platform}</h3>
                                <div className="platform-count">
                                    {gamesByPlatform[platform].length} Games
                                </div>
                            </div>
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
                        {gamesByPlatform[selectedPlatform]?.map((game) => (
                            <div key={game.id} className="card game-card">
                                <GameCover game={game} className="game-cover" />
                                <h3>{game.name}</h3>
                            </div>
                        ))}
                    </div>
                </>
            )}
        </div>
    );
}

export default Library;
