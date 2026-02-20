import { useState, useEffect } from 'react';
import { GetRom, DownloadRomToLibrary, GetRomDownloadStatus, DeleteRom, PlayRom } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';

const TrashIcon = ({ size = 24 }: { size?: number }) => (
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
        <path d="M3 6h18" />
        <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
        <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
        <line x1="10" y1="11" x2="10" y2="17" />
        <line x1="14" y1="11" x2="14" y2="17" />
    </svg>
);

interface GamePageProps {
    gameId: number;
    onBack: () => void;
}

export function GamePage({ gameId, onBack }: GamePageProps) {
    const [game, setGame] = useState<types.Game | null>(null);
    const [loading, setLoading] = useState(true);
    const [downloading, setDownloading] = useState(false);
    const [downloadStatus, setDownloadStatus] = useState<string | null>(null);
    const [isDownloaded, setIsDownloaded] = useState(false);
    const [statusChecked, setStatusChecked] = useState(false);

    const { ref } = useFocusable({
        onArrowPress: (direction: string) => {
            // Internal navigation could go here if we have more buttons
            return true;
        },
    });

    const { ref: downloadRef, focused: downloadFocused, focusSelf: focusDownload } = useFocusable({
        focusKey: 'download-button',
        onEnterPress: () => {
            if (game && !downloading && !isDownloaded) {
                setDownloading(true);
                setDownloadStatus("Starting download...");
                DownloadRomToLibrary(game.id)
                    .then(() => {
                        setDownloadStatus("Download complete!");
                        setIsDownloaded(true);
                        setTimeout(() => {
                            setFocus('play-button');
                        }, 100);
                    })
                    .catch((err: string) => {
                        setDownloadStatus(`Error: ${err}`);
                    })
                    .finally(() => {
                        setDownloading(false);
                    });
            }
        },
    });

    const { ref: playRef, focused: playFocused, focusSelf: focusPlay } = useFocusable({
        focusKey: 'play-button',
        onEnterPress: () => {
            if (game) {
                setDownloadStatus("Starting RetroArch...");
                PlayRom(game.id).then(() => {
                    setDownloadStatus("Game launched successfully!");
                }).catch((err: string) => {
                    // Check if it's the RetroArch cancelled error
                    if (err.includes("launch cancelled")) {
                        setDownloadStatus("");
                    } else {
                        setDownloadStatus(`Play error: ${err}`);
                    }
                });
            }
        },
    });

    const { ref: deleteRef, focused: deleteFocused, focusSelf: focusDelete } = useFocusable({
        focusKey: 'delete-button',
        onEnterPress: () => {
            if (!game) return;

            DeleteRom(game.id).then(() => {
                setIsDownloaded(false);
                setDownloadStatus("ROM deleted from library.");
                setTimeout(() => focusDownload(), 100);
            }).catch((err: any) => {
                setDownloadStatus(`Delete error: ${err}`);
            });
        },
    });

    useEffect(() => {
        GetRom(gameId)
            .then((res: types.Game) => {
                setGame(res);
                // Check if already downloaded
                GetRomDownloadStatus(gameId).then((status: boolean) => {
                    setIsDownloaded(status);
                    setStatusChecked(true);
                    // Set focus to the primary button after data is loaded
                    setTimeout(() => {
                        if (status) {
                            setFocus('play-button');
                        } else {
                            setFocus('download-button');
                        }
                    }, 100);
                }).catch(() => {
                    setStatusChecked(true); // Still mark as checked even on error
                });
            })
            .catch((err: any) => {
                setDownloadStatus(`Error fetching game: ${err}`);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [gameId]);

    if (loading) {
        return <div className="game-page-loading">Loading game details...</div>;
    }

    if (!game) {
        return <div className="game-page-error">Game not found.</div>;
    }

    return (
        <div id="game-page" ref={ref}>
            <div className="game-page-content">
                <div className="game-sidebar">
                    <GameCover game={game} className="game-page-cover" />
                    {statusChecked && (
                        !isDownloaded ? (
                            <button
                                ref={downloadRef}
                                className={`btn download-btn ${downloadFocused ? 'focused' : ''} ${downloading ? 'disabled' : ''}`}
                                onMouseEnter={() => {
                                    if (getMouseActive()) {
                                        focusDownload();
                                    }
                                }}
                                onClick={() => {
                                    if (!downloading) {
                                        setDownloading(true);
                                        setDownloadStatus("Starting download...");
                                        DownloadRomToLibrary(game.id)
                                            .then(() => {
                                                setDownloadStatus("Download complete!");
                                                setIsDownloaded(true);
                                                setTimeout(() => {
                                                    setFocus('play-button');
                                                }, 100);
                                            })
                                            .catch((err: string) => {
                                                setDownloadStatus(`Error: ${err}`);
                                            })
                                            .finally(() => {
                                                setDownloading(false);
                                            });
                                    }
                                }}
                            >
                                {downloading ? "Downloading..." : "Download to Library"}
                            </button>
                        ) : (
                            <div className="game-actions-horizontal">
                                <button
                                    ref={playRef}
                                    className={`btn play-btn ${playFocused ? 'focused' : ''}`}
                                    onMouseEnter={() => {
                                        if (getMouseActive()) {
                                            focusPlay();
                                        }
                                    }}
                                    onClick={() => {
                                        if (game) {
                                            setDownloadStatus("Starting RetroArch...");
                                            PlayRom(game.id).then(() => {
                                                setDownloadStatus("Game launched successfully!");
                                            }).catch((err: string) => {
                                                if (err.includes("launch cancelled")) {
                                                    setDownloadStatus("");
                                                } else {
                                                    setDownloadStatus(`Play error: ${err}`);
                                                }
                                            });
                                        }
                                    }}
                                >
                                    Play
                                </button>
                                <button
                                    ref={deleteRef}
                                    className={`btn delete-btn ${deleteFocused ? 'focused' : ''}`}
                                    title="Delete ROM"
                                    onMouseEnter={() => {
                                        if (getMouseActive()) {
                                            focusDelete();
                                        }
                                    }}
                                    onClick={() => {
                                        if (!game) return;

                                        DeleteRom(game.id).then(() => {
                                            setIsDownloaded(false);
                                            setDownloadStatus("ROM deleted from library.");
                                            setTimeout(() => focusDownload(), 100);
                                        }).catch((err: any) => {
                                            setDownloadStatus(`Delete error: ${err}`);
                                        });
                                    }}
                                >
                                    <TrashIcon />
                                </button>
                            </div>
                        )
                    )}
                    {downloadStatus && (
                        <div className={`download-status ${downloadStatus.includes("Error") ? "error" : "success"}`}>
                            {downloadStatus}
                        </div>
                    )}
                </div>
                <div className="game-main-info">
                    <h1>{game.name}</h1>
                    {game.genres && game.genres.length > 0 && (
                        <div className="game-genres">
                            {game.genres.map((genre: string, idx: number) => (
                                <span key={idx} className="genre-tag">{genre}</span>
                            ))}
                        </div>
                    )}
                    <div className="game-summary">
                        <h3>Summary</h3>
                        <p>{game.summary || "No summary available."}</p>
                    </div>
                    {game.has_saves && (
                        <div className="game-saves-status">
                            <span className="save-icon">💾</span> Saves available
                        </div>
                    )}
                </div>
            </div>

            <div className="input-legend">
                <div className="footer-left">
                    <span>{game.name}</span>
                </div>
                <div className="footer-right">
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

export default GamePage;
