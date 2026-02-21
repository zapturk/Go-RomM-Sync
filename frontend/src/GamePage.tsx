import { useState, useEffect } from 'react';
import { GetRom, DownloadRomToLibrary, GetRomDownloadStatus, DeleteRom, PlayRom, GetSaves, GetStates, DeleteSave, DeleteState } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
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

const SaveIcon = ({ size = 20 }: { size?: number }) => (
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
        <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v13a2 2 0 0 1-2 2z" />
        <polyline points="17 21 17 13 7 13 7 21" />
        <polyline points="7 3 7 8 15 8" />
    </svg>
);

const FileItemRow = ({ item, onDelete, focusKey }: { item: any, onDelete: () => void, focusKey: string }) => {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress: onDelete,
    });

    return (
        <div className={`file-item-row ${focused ? 'focused' : ''}`} ref={ref}>
            <span className="file-name" title={item.name}>{item.name}</span>
            <span className="file-core">{item.core}</span>
            <button
                className="file-delete-btn"
                onClick={(e) => {
                    e.stopPropagation();
                    onDelete();
                }}
            >
                <TrashIcon size={16} />
            </button>
        </div>
    );
};


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
    const [saves, setSaves] = useState<any[]>([]);
    const [states, setStates] = useState<any[]>([]);

    const { ref } = useFocusable({
        onArrowPress: (direction: string) => {
            // Internal navigation could go here if we have more buttons
            return true;
        },
    });

    const { ref: downloadRef, focused: downloadFocused, focusSelf: focusDownload } = useFocusable({
        focusKey: 'download-button',
        onArrowPress: (direction: string) => direction === 'down' || direction === 'right', // Allow moving down to Saves/States or right to Delete
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
        onArrowPress: (direction: string) => direction === 'right' || direction === 'down', // Allow moving to Delete or down to Saves/States
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
        onArrowPress: (direction: string) => direction === 'left' || direction === 'down' || direction === 'right', // Allow moving back to Play, down to Saves/States, or right to Saves/States
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

                // Fetch saves and states
                fetchAppData();
            })
            .catch((err: any) => {
                setDownloadStatus(`Error fetching game: ${err}`);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [gameId]);

    useEffect(() => {
        const unsubscribe = EventsOn("game-exited", () => {
            fetchAppData();
        });
        return () => unsubscribe();
    }, [gameId]);

    const fetchAppData = () => {
        GetSaves(gameId).then(res => setSaves(res || [])).catch(console.error);
        GetStates(gameId).then(res => setStates(res || [])).catch(console.error);
    };

    const handleDeleteSave = (core: string, name: string, index: number) => {
        DeleteSave(gameId, core, name).then(() => {
            GetSaves(gameId).then(res => {
                const newSaves = res || [];
                setSaves(newSaves);
                setTimeout(() => {
                    if (newSaves.length > 0) {
                        const nextIdx = Math.min(index, newSaves.length - 1);
                        setFocus(`save-${nextIdx}`);
                    } else if (states.length > 0) {
                        setFocus(`state-0`);
                    } else {
                        setFocus('play-button');
                    }
                }, 50);
            }).catch(console.error);
            setDownloadStatus("Save deleted.");
        }).catch(err => setDownloadStatus(`Error deleting save: ${err}`));
    };

    const handleDeleteState = (core: string, name: string, index: number) => {
        DeleteState(gameId, core, name).then(() => {
            GetStates(gameId).then(res => {
                const newStates = res || [];
                setStates(newStates);
                setTimeout(() => {
                    if (newStates.length > 0) {
                        const nextIdx = Math.min(index, newStates.length - 1);
                        setFocus(`state-${nextIdx}`);
                    } else if (saves.length > 0) {
                        setFocus(`save-0`);
                    } else {
                        setFocus('play-button');
                    }
                }, 50);
            }).catch(console.error);
            setDownloadStatus("State deleted.");
        }).catch(err => setDownloadStatus(`Error deleting state: ${err}`));
    };

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
                    <div className="game-saves-states-section">
                        <div className="game-saves-column">
                            <h3>Saves</h3>
                            <div className="file-list">
                                {saves.map((save, idx) => (
                                    <FileItemRow
                                        key={`save-${idx}`}
                                        focusKey={`save-${idx}`}
                                        item={save}
                                        onDelete={() => handleDeleteSave(save.core, save.name, idx)}
                                    />
                                ))}
                                {saves.length === 0 && <p className="no-files">No saves found.</p>}
                            </div>
                        </div>
                        <div className="game-states-column">
                            <h3>States</h3>
                            <div className="file-list">
                                {states.map((state, idx) => (
                                    <FileItemRow
                                        key={`state-${idx}`}
                                        focusKey={`state-${idx}`}
                                        item={state}
                                        onDelete={() => handleDeleteState(state.core, state.name, idx)}
                                    />
                                ))}
                                {states.length === 0 && <p className="no-files">No states found.</p>}
                            </div>
                        </div>
                    </div>
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
