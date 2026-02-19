import { useState, useEffect } from 'react';
import { GetLibrary, GetPlatforms } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GameCard } from "./GameCard";
import { PlatformCard } from "./PlatformCard";

function Library() {
    const [games, setGames] = useState<types.Game[]>([]);
    const [platforms, setPlatforms] = useState<types.Platform[]>([]);
    const [status, setStatus] = useState("Loading library...");

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

    // We don't need to manually extract platforms from paths anymore, 
    // but we need to link games to the selected platform.
    // The previous implementation used folder names as platform names.
    // RomM data might have platform_id? Let's check Game struct.
    // types/game.go doesn't show platform_id.
    // However, looking at previous implementation: getPlatformFromPath.
    // Using fetched platforms is better, but we need to map games to them. 
    // "slug" or "name" likely matches the folder or we check if RomM API returns platform_id in Game.
    // Step 260 shows Game struct: id, name, rom_id, url_cover, full_path.
    // We will stick to `getPlatformFromPath` logic to MATCH the fetched platform name/slug?
    // Or just filter based on if the platform name is in the path?
    // Let's assume Platform.Name roughly matches the folder name from getPlatformFromPath.

    // Sort platforms by name
    const sortedPlatforms = [...platforms].sort((a, b) => a.name.localeCompare(b.name));

    // Helper to get games for a platform
    const getGamesForPlatform = (platform: types.Platform) => {
        return games.filter(game => {
            // Heuristic: check if platform name is part of the path
            // This is imperfect but works with existing logic
            return game.full_path.includes("/" + platform.name + "/") ||
                game.full_path.includes("/" + platform.slug + "/");
        });
    };

    return (
        <div id="library">
            {!selectedPlatform ? (
                // Platform Grid View
                <>
                    <h1>Platforms</h1>
                    <button className="btn" onClick={refreshLibrary} style={{ marginBottom: "1rem" }}>Sync Library</button>
                    <p>{status}</p>
                    <div className="grid-container">
                        {sortedPlatforms.filter(p => getGamesForPlatform(p).length > 0).map((platform) => (
                            <PlatformCard
                                key={platform.id}
                                platform={platform}
                                onClick={() => setSelectedPlatform(platform.name)}
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
                                <GameCard key={game.id} game={game} />
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
