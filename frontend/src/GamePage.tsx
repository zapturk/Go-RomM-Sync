import { useState, useEffect, useRef } from 'react';
import { GetRom, DownloadRomToLibrary, GetRomDownloadStatus, DeleteRom, PlayRom, GetSaves, GetStates, DeleteSave, DeleteState, UploadSave, UploadState, GetServerSaves, GetServerStates, DownloadServerSave, DownloadServerState } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";
import { TrashIcon } from "./components/Icons";
import { FileItemRow } from "./FileItemRow";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { TIMESTAMP_REGEX, APP_EVENTS } from './constants';

const decodeHtml = (html: string) => {
    if (!html) return '';
    const txt = document.createElement("textarea");
    txt.innerHTML = html;
    return txt.value;
};

interface GamePageProps {
    gameId: number;
    onBack: () => void;
}

const formatFileSize = (bytes: number) => {
    if (!bytes || bytes === 0) return '';
    const MB = 1024 * 1024;
    const GB = 1024 * MB;

    if (bytes < MB) {
        return `File Size: ${bytes.toLocaleString()} Bytes`;
    } else if (bytes < GB) {
        return `File Size: ${(bytes / MB).toFixed(2)} MB`;
    } else {
        return `File Size: ${(bytes / GB).toFixed(2)} GB`;
    }
};

const getFileStatus = (item: any, otherList: any[]) => {
    const name = item.name || item.file_name;
    const core = item.core || item.emulator;
    if (!name || !core || !item.updated_at) return undefined;

    const other = otherList.find(o => (o.name === name || o.file_name === name) && (o.core === core || o.emulator === core));
    if (!other || !other.updated_at) return undefined;

    const currentDate = new Date(item.updated_at).getTime();
    const otherDate = new Date(other.updated_at).getTime();

    const diff = currentDate - otherDate;
    if (Math.abs(diff) < 5000) return 'equal'; // 5s buffer for clock drift/transfer latency
    return diff > 0 ? 'newer' : 'older';
};

export function GamePage({ gameId, onBack }: GamePageProps) {
    const [game, setGame] = useState<types.Game | null>(null);
    const [loading, setLoading] = useState(true);
    const [downloading, setDownloading] = useState(false);
    const [downloadStatus, setDownloadStatus] = useState<string | null>(null);
    const [isDownloaded, setIsDownloaded] = useState(false);
    const [statusChecked, setStatusChecked] = useState(false);
    const [saves, setSaves] = useState<types.FileItem[]>([]);
    const [states, setStates] = useState<types.FileItem[]>([]);
    const [serverSaves, setServerSaves] = useState<types.ServerSave[]>([]);
    const [serverStates, setServerStates] = useState<types.ServerState[]>([]);
    const [downloadProgress, setDownloadProgress] = useState<number>(0);
    const [statusFading, setStatusFading] = useState(false);
    const [isPlaying, setIsPlaying] = useState(false);
    const fadeTimeoutRef = useRef<any>(null);
    const clearStatusTimeoutRef = useRef<any>(null);

    const setSuccessStatus = (msg: string) => {
        // Clear any existing timeouts to reset the cycle
        if (fadeTimeoutRef.current) clearTimeout(fadeTimeoutRef.current);
        if (clearStatusTimeoutRef.current) clearTimeout(clearStatusTimeoutRef.current);

        setStatusFading(false);
        setDownloadStatus(msg);

        // Start fading after 1 second (opaque for 1s, then 2s fade = 3s total)
        fadeTimeoutRef.current = setTimeout(() => {
            setStatusFading(true);
        }, 1000);

        // Clear status after 3 seconds
        clearStatusTimeoutRef.current = setTimeout(() => {
            setDownloadStatus(prev => prev === msg ? null : prev);
            setStatusFading(false);
        }, 3000);
    };

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (fadeTimeoutRef.current) clearTimeout(fadeTimeoutRef.current);
            if (clearStatusTimeoutRef.current) clearTimeout(clearStatusTimeoutRef.current);
        };
    }, []);

    useEffect(() => {
        const unlisten = EventsOn("download-progress", (data: { game_id: number; percentage: number }) => {
            if (data.game_id === gameId) {
                setDownloadProgress(data.percentage);
            }
        });

        const unlistenStarted = EventsOn(APP_EVENTS.GAME_STARTED, () => setIsPlaying(true));
        const unlistenExited = EventsOn(APP_EVENTS.GAME_EXITED, () => setIsPlaying(false));

        return () => {
            unlisten();
            unlistenStarted();
            unlistenExited();
        };
    }, [gameId]);

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
                        setSuccessStatus("Download complete!");
                        setDownloadProgress(100);
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
                    setSuccessStatus("Game launched successfully!");
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
                setSuccessStatus("ROM deleted from library.");
                setTimeout(() => focusDownload(), 100);
            }).catch((err: string) => {
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
            .catch((err: string) => {
                setDownloadStatus(`Error fetching game: ${err}`);
            })
            .finally(() => {
                setLoading(false);
            });
    }, [gameId]);

    useEffect(() => {
        const unsubscribe = EventsOn(APP_EVENTS.GAME_EXITED, () => {
            fetchAppData();
        });
        return () => unsubscribe();
    }, [gameId]);

    const fetchAppData = () => {
        GetSaves(gameId).then(res => setSaves(res || [])).catch(console.error);
        GetStates(gameId).then(res => setStates(res || [])).catch(console.error);
        GetServerSaves(gameId).then(res => setServerSaves(res || [])).catch(console.error);
        GetServerStates(gameId).then(res => setServerStates(res || [])).catch(console.error);
    };

    const handleDeleteSave = (core: string, name: string, index: number) => {
        DeleteSave(gameId, core, name).then(() => {
            GetSaves(gameId).then(res => {
                const newSaves = res || [];
                setSaves(newSaves);
                setTimeout(() => {
                    if (newSaves.length > 0) {
                        const nextIdx = Math.min(index, newSaves.length - 1);
                        setFocus(`save-${nextIdx}-upload`);
                    } else if (states.length > 0) {
                        setFocus(`state-0-upload`);
                    } else if (isDownloaded) {
                        setFocus('play-button');
                    } else {
                        setFocus('download-button');
                    }
                }, 50);
            }).catch(console.error);
            setSuccessStatus("Save deleted.");
        }).catch((err: string) => setDownloadStatus(`Error deleting save: ${err}`));
    };

    const handleDeleteState = (core: string, name: string, index: number) => {
        DeleteState(gameId, core, name).then(() => {
            GetStates(gameId).then(res => {
                const newStates = res || [];
                setStates(newStates);
                setTimeout(() => {
                    if (newStates.length > 0) {
                        const nextIdx = Math.min(index, newStates.length - 1);
                        setFocus(`state-${nextIdx}-upload`);
                    } else if (saves.length > 0) {
                        setFocus(`save-0-upload`);
                    } else if (isDownloaded) {
                        setFocus('play-button');
                    } else {
                        setFocus('download-button');
                    }
                }, 50);
            }).catch(console.error);
            setSuccessStatus("State deleted.");
        }).catch((err: string) => setDownloadStatus(`Error deleting state: ${err}`));
    };

    const handleUploadSave = (core: string, name: string) => {
        setDownloadStatus(`Uploading save ${name}...`);
        UploadSave(gameId, core, name).then(() => {
            setSuccessStatus("Save uploaded successfully to RomM!");
            fetchAppData(); // Refresh server save list
        }).catch((err: string) => {
            setDownloadStatus(`Upload error: ${err}`);
        });
    };

    const handleUploadState = (core: string, name: string) => {
        setDownloadStatus(`Uploading state ${name}...`);
        UploadState(gameId, core, name).then(() => {
            setSuccessStatus("State uploaded successfully to RomM!");
            fetchAppData(); // Refresh server state list
        }).catch((err: string) => {
            setDownloadStatus(`Upload error: ${err}`);
        });
    };

    const handleDownloadServerSave = (save: types.ServerSave) => {
        setDownloadStatus(`Downloading save ${save.file_name}...`);
        const cleanFileName = save.file_name.replace(TIMESTAMP_REGEX, "");
        DownloadServerSave(gameId, save.full_path, save.emulator, cleanFileName, save.updated_at).then(() => {
            setSuccessStatus("Server save downloaded successfully!");
            fetchAppData(); // Refresh local saves list
        }).catch((err: string) => {
            setDownloadStatus(`Download error: ${err}`);
        });
    };

    const handleDownloadServerState = (state: types.ServerState) => {
        setDownloadStatus(`Downloading state ${state.file_name}...`);
        const cleanFileName = state.file_name.replace(TIMESTAMP_REGEX, "");
        DownloadServerState(gameId, state.full_path, state.emulator, cleanFileName, state.updated_at).then(() => {
            setSuccessStatus("Server state downloaded successfully!");
            fetchAppData(); // Refresh local states list
        }).catch((err: string) => {
            setDownloadStatus(`Download error: ${err}`);
        });
    };

    const handleSyncSaves = async () => {
        setDownloadStatus("Starting smart sync for saves...");
        const allNames = new Set<string>();
        saves.forEach(s => allNames.add(s.name));
        serverSaves.forEach(s => {
            const cleanName = s.file_name.replace(TIMESTAMP_REGEX, "");
            allNames.add(cleanName);
        });

        for (const name of Array.from(allNames)) {
            const local = saves.find(s => s.name === name);
            const serverClean = serverSaves.find(s => s.file_name.replace(TIMESTAMP_REGEX, "") === name);

            if (local && serverClean) {
                const localTime = new Date(local.updated_at || "").getTime();
                const serverTime = new Date(serverClean.updated_at || "").getTime();
                if (localTime > serverTime) {
                    await UploadSave(gameId, local.core, local.name).catch(console.error);
                } else if (serverTime > localTime) {
                    await DownloadServerSave(gameId, serverClean.full_path, serverClean.emulator, name, serverClean.updated_at).catch(console.error);
                }
            } else if (local && !serverClean) {
                await UploadSave(gameId, local.core, local.name).catch(console.error);
            } else if (!local && serverClean) {
                await DownloadServerSave(gameId, serverClean.full_path, serverClean.emulator, name, serverClean.updated_at).catch(console.error);
            }
        }
        setSuccessStatus("Smart sync for saves complete!");
        fetchAppData();
    };

    const handleSyncStates = async () => {
        setDownloadStatus("Starting smart sync for states...");
        const allNames = new Set<string>();
        states.forEach(s => allNames.add(s.name));
        serverStates.forEach(s => {
            const cleanName = s.file_name.replace(TIMESTAMP_REGEX, "");
            allNames.add(cleanName);
        });

        for (const name of Array.from(allNames)) {
            const local = states.find(s => s.name === name);
            const serverClean = serverStates.find(s => s.file_name.replace(TIMESTAMP_REGEX, "") === name);

            if (local && serverClean) {
                const localTime = new Date(local.updated_at || "").getTime();
                const serverTime = new Date(serverClean.updated_at || "").getTime();
                if (localTime > serverTime) {
                    await UploadState(gameId, local.core, local.name).catch(console.error);
                } else if (serverTime > localTime) {
                    await DownloadServerState(gameId, serverClean.full_path, serverClean.emulator, name, serverClean.updated_at).catch(console.error);
                }
            } else if (local && !serverClean) {
                await UploadState(gameId, local.core, local.name).catch(console.error);
            } else if (!local && serverClean) {
                await DownloadServerState(gameId, serverClean.full_path, serverClean.emulator, name, serverClean.updated_at).catch(console.error);
            }
        }
        setSuccessStatus("Smart sync for states complete!");
        fetchAppData();
    };

    const handleSmartSync = async () => {
        setDownloadStatus("Starting full smart sync...");
        await handleSyncSaves();
        await handleSyncStates();
        setSuccessStatus("Smart sync complete!");
    };

    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            // Don't trigger sync if we're typing in an input
            const activeElement = document.activeElement;
            const isTyping = activeElement?.tagName === 'INPUT' || activeElement?.tagName === 'TEXTAREA';

            if (!isTyping && e.key.toLowerCase() === 'r') {
                handleSmartSync();
            }
        };

        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [saves, serverSaves, states, serverStates, gameId]);

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
                    {game.fs_size_bytes > 0 && (
                        <div className="game-file-size">
                            {formatFileSize(game.fs_size_bytes)}
                        </div>
                    )}
                    {statusChecked && (
                        !isDownloaded ? (
                            <button
                                ref={downloadRef}
                                className={`btn download-btn ${downloadFocused ? 'focused' : ''} ${downloading || isPlaying ? 'disabled' : ''}`}
                                onMouseEnter={() => {
                                    if (getMouseActive() && !downloading && !isPlaying) {
                                        focusDownload();
                                    }
                                }}
                                onClick={() => {
                                    if (!downloading && !isPlaying) {
                                        setDownloading(true);
                                        setDownloadStatus("Starting download...");
                                        DownloadRomToLibrary(game.id)
                                            .then(() => {
                                                setSuccessStatus("Download complete!");
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
                                    className={`btn play-btn ${playFocused ? 'focused' : ''} ${isPlaying ? 'disabled' : ''}`}
                                    onMouseEnter={() => {
                                        if (getMouseActive() && !isPlaying) {
                                            focusPlay();
                                        }
                                    }}
                                    onClick={() => {
                                        if (game && !isPlaying) {
                                            setDownloadStatus("Starting RetroArch...");
                                            PlayRom(game.id).then(() => {
                                                setSuccessStatus("Game launched successfully!");
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
                                    className={`btn delete-btn ${deleteFocused ? 'focused' : ''} ${isPlaying ? 'disabled' : ''}`}
                                    title="Delete ROM"
                                    onMouseEnter={() => {
                                        if (getMouseActive() && !isPlaying) {
                                            focusDelete();
                                        }
                                    }}
                                    onClick={() => {
                                        if (!game || isPlaying) return;

                                        DeleteRom(game.id).then(() => {
                                            setIsDownloaded(false);
                                            setSuccessStatus("ROM deleted from library.");
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
                        <div className={`download-status ${downloadStatus.includes("Error") ? "error" : "success"} ${statusFading && !downloadStatus.includes("Error") ? "fading" : ""}`}>
                            {downloadStatus}
                            {downloading && downloadProgress > 0 && downloadProgress < 100 && (
                                <span style={{ marginLeft: "10px", fontWeight: "bold" }}>
                                    ({Math.round(downloadProgress)}%)
                                </span>
                            )}
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
                        <p>{decodeHtml(game.summary) || "No summary available."}</p>
                    </div>
                    <div className="game-saves-states-section">
                        <div className="game-saves-column">
                            <h3>Server Saves</h3>
                            <div className="file-list">
                                {serverSaves.map((save, idx) => (
                                    <FileItemRow
                                        key={`server-save-${idx}`}
                                        focusKeyPrefix={`server-save-${idx}`}
                                        item={save}
                                        onDownload={() => handleDownloadServerSave(save)}
                                        status={getFileStatus(save, saves)}
                                        isDisabled={isPlaying}
                                    />
                                ))}
                                {serverSaves.length === 0 && <p className="no-files">No server saves found.</p>}
                            </div>

                            <h3 style={{ marginTop: '20px' }}>Local Saves</h3>
                            <div className="file-list">
                                {saves.map((save, idx) => (
                                    <FileItemRow
                                        key={`save-${idx}`}
                                        focusKeyPrefix={`save-${idx}`}
                                        item={save}
                                        onDelete={() => handleDeleteSave(save.core, save.name, idx)}
                                        onUpload={() => handleUploadSave(save.core, save.name)}
                                        status={getFileStatus(save, serverSaves)}
                                        isDisabled={isPlaying}
                                    />
                                ))}
                                {saves.length === 0 && <p className="no-files">No local saves found.</p>}
                            </div>
                        </div>
                        <div className="game-states-column">
                            <h3>Server States</h3>
                            <div className="file-list">
                                {serverStates.map((state, idx) => (
                                    <FileItemRow
                                        key={`server-state-${idx}`}
                                        focusKeyPrefix={`server-state-${idx}`}
                                        item={state}
                                        onDownload={() => handleDownloadServerState(state)}
                                        status={getFileStatus(state, states)}
                                        isDisabled={isPlaying}
                                    />
                                ))}
                                {serverStates.length === 0 && <p className="no-files">No server states found.</p>}
                            </div>

                            <h3 style={{ marginTop: '20px' }}>Local States</h3>
                            <div className="file-list">
                                {states.map((state, idx) => (
                                    <FileItemRow
                                        key={`state-${idx}`}
                                        focusKeyPrefix={`state-${idx}`}
                                        item={state}
                                        onDelete={() => handleDeleteState(state.core, state.name, idx)}
                                        onUpload={() => handleUploadState(state.core, state.name)}
                                        status={getFileStatus(state, serverStates)}
                                        isDisabled={isPlaying}
                                    />
                                ))}
                                {states.length === 0 && <p className="no-files">No local states found.</p>}
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
                            <div className="btn-dot east"></div>
                            <div className="btn-dot south"></div>
                            <div className="btn-dot west active"></div>
                        </div>
                        <div className="key-icon show-keyboard">R</div>
                        <span>Sync</span>
                    </div>

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
