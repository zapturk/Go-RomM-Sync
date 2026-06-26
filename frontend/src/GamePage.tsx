import { useState, useEffect, useRef, useCallback } from 'react';
import { GetRom, DownloadRomToLibrary, GetRomDownloadStatus, DeleteRom, PlayRomWithCore, GetCoresForGame,
    GetSaves, GetStates, DeleteSave, DeleteState, UploadSave, UploadState,
    GetServerSaves, GetServerStates, DownloadServerSave, DownloadServerState,
    OpenGameFolder, GetFirmware, SetPlatformFirmware, GetConfig, CancelDownload,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";
import { TrashIcon, FolderIcon, PlayIcon, DownloadIcon } from "./components/Icons";
import { FileItemRow, getItemName, getItemCore } from "./FileItemRow";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { TIMESTAMP_REGEX, APP_EVENTS } from './constants';
import { LegendItem } from './components/LegendItem';

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



const isMatchingItem = (o: any, name: string, core: string) => 
    getItemName(o) === name && getItemCore(o) === core;

const getItemTime = (item: any): number => {
    if (!item || !item.updated_at) return 0;
    return new Date(item.updated_at).getTime();
};

const getFileStatus = (item: any, otherList: any[]) => {
    const name = getItemName(item);
    const core = getItemCore(item);
    const itemTime = getItemTime(item);
    if (!name || !core || !itemTime) return undefined;

    const other = otherList.find(o => isMatchingItem(o, name, core));
    const otherTime = getItemTime(other);
    if (!otherTime) return undefined;

    const diff = itemTime - otherTime;
    if (Math.abs(diff) < 5000) return 'equal'; // 5s buffer for clock drift/transfer latency
    return diff > 0 ? 'newer' : 'older';
};

const getTargetFirmware = (firmwares: types.Firmware[], id: number): types.Firmware | null => {
    if (id === 0) {
        return { id: 0, platform_id: 0, file_name: '', md5_hash: '', file_size_bytes: 0, is_verified: false } as unknown as types.Firmware;
    }
    return firmwares.find(f => f.id === id) || null;
};

enum SyncAction {
    Upload,
    Download,
    None
}

const determineSyncAction = (local?: any, server?: any): SyncAction => {
    if (!local) {
        return server ? SyncAction.Download : SyncAction.None;
    }
    if (!server) {
        return SyncAction.Upload;
    }
    const localTime = getItemTime(local);
    const serverTime = getItemTime(server);
    if (localTime > serverTime) return SyncAction.Upload;
    if (serverTime > localTime) return SyncAction.Download;
    return SyncAction.None;
};

const handleEscapeKey = (
    e: KeyboardEvent,
    isPickerOpen: boolean,
    isFirmwarePickerOpen: boolean,
    closePicker: () => void,
    closeFirmwarePicker: () => void
): boolean => {
    if (e.key !== 'Escape') return false;
    if (isPickerOpen) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closePicker();
        return true;
    }
    if (isFirmwarePickerOpen) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closeFirmwarePicker();
        return true;
    }
    return false;
};

function useGameSavesAndStates(
    gameId: number,
    offlineMode: boolean,
    isDownloaded: boolean,
    setDownloadStatus: (status: string | null) => void,
    setSuccessStatus: (msg: string) => void
) {
    const [saves, setSaves] = useState<types.FileItem[]>([]);
    const [states, setStates] = useState<types.FileItem[]>([]);
    const [serverSaves, setServerSaves] = useState<types.ServerSave[]>([]);
    const [serverStates, setServerStates] = useState<types.ServerState[]>([]);

    const fetchAppData = useCallback(() => {
        GetSaves(gameId).then(res => setSaves(res || [])).catch(console.error);
        GetStates(gameId).then(res => setStates(res || [])).catch(console.error);
        GetServerSaves(gameId).then(res => setServerSaves(res || [])).catch(console.error);
        GetServerStates(gameId).then(res => setServerStates(res || [])).catch(console.error);
    }, [gameId]);

    const focusFallbackAfterDeletion = useCallback((
        primaryList: any[],
        secondaryList: any[],
        index: number,
        primaryPrefix: 'save' | 'state',
        secondaryPrefix: 'save' | 'state'
    ) => {
        if (primaryList.length > 0) {
            const nextIdx = Math.min(index, primaryList.length - 1);
            setFocus(`${primaryPrefix}-${nextIdx}-upload`);
        } else if (secondaryList.length > 0) {
            setFocus(`${secondaryPrefix}-0-upload`);
        } else if (isDownloaded) {
            setFocus('play-button');
        } else {
            setFocus('download-button');
        }
    }, [isDownloaded]);

    const syncFiles = useCallback(async (
        type: 'saves' | 'states',
        localList: any[],
        serverList: any[],
        uploadFn: (gameId: number, core: string, name: string) => Promise<any>,
        downloadFn: (gameId: number, id: number, emulator: string, name: string, updatedAt: string) => Promise<any>
    ) => {
        setDownloadStatus(`Starting smart sync for ${type}...`);
        const allNames = new Set<string>();
        localList.forEach(s => allNames.add(s.name));
        serverList.forEach(s => {
            const cleanName = s.file_name.replace(TIMESTAMP_REGEX, "");
            allNames.add(cleanName);
        });

        for (const name of Array.from(allNames)) {
            const local = localList.find(s => s.name === name);
            const serverClean = serverList.find(s => s.file_name.replace(TIMESTAMP_REGEX, "") === name);

            const action = determineSyncAction(local, serverClean);
            if (action === SyncAction.Upload && local) {
                await uploadFn(gameId, local.core, local.name).catch(console.error);
            } else if (action === SyncAction.Download && serverClean) {
                await downloadFn(gameId, serverClean.id, serverClean.emulator, name, serverClean.updated_at).catch(console.error);
            }
        }
        setSuccessStatus(`Smart sync for ${type} complete!`);
        fetchAppData();
    }, [gameId, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleDeleteSave = useCallback((core: string, name: string, index: number) => {
        DeleteSave(gameId, core, name).then(() => {
            GetSaves(gameId).then(res => {
                const newSaves = res || [];
                setSaves(newSaves);
                setTimeout(() => focusFallbackAfterDeletion(newSaves, states, index, 'save', 'state'), 50);
            }).catch(console.error);
            setSuccessStatus("Save deleted.");
        }).catch((err: string) => setDownloadStatus(`Error deleting save: ${err}`));
    }, [gameId, states, focusFallbackAfterDeletion, setSuccessStatus, setDownloadStatus]);

    const handleDeleteState = useCallback((core: string, name: string, index: number) => {
        DeleteState(gameId, core, name).then(() => {
            GetStates(gameId).then(res => {
                const newStates = res || [];
                setStates(newStates);
                setTimeout(() => focusFallbackAfterDeletion(newStates, saves, index, 'state', 'save'), 50);
            }).catch(console.error);
            setSuccessStatus("State deleted.");
        }).catch((err: string) => setDownloadStatus(`Error deleting state: ${err}`));
    }, [gameId, saves, focusFallbackAfterDeletion, setSuccessStatus, setDownloadStatus]);

    const handleUploadSave = useCallback((core: string, name: string) => {
        setDownloadStatus(`Uploading save ${name}...`);
        UploadSave(gameId, core, name).then(() => {
            setSuccessStatus("Save uploaded successfully to RomM!");
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Upload error: ${err}`);
        });
    }, [gameId, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleUploadState = useCallback((core: string, name: string) => {
        setDownloadStatus(`Uploading state ${name}...`);
        UploadState(gameId, core, name).then(() => {
            setSuccessStatus("State uploaded successfully to RomM!");
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Upload error: ${err}`);
        });
    }, [gameId, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleDownloadServerSave = useCallback((save: types.ServerSave) => {
        setDownloadStatus(`Downloading save ${save.file_name}...`);
        const cleanFileName = save.file_name.replace(TIMESTAMP_REGEX, "");
        DownloadServerSave(gameId, save.id, save.emulator, cleanFileName, save.updated_at).then(() => {
            setSuccessStatus("Server save downloaded successfully!");
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Download error: ${err}`);
        });
    }, [gameId, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleDownloadServerState = useCallback((state: types.ServerState) => {
        setDownloadStatus(`Downloading state ${state.file_name}...`);
        const cleanFileName = state.file_name.replace(TIMESTAMP_REGEX, "");
        DownloadServerState(gameId, state.id, state.emulator, cleanFileName, state.updated_at).then(() => {
            setSuccessStatus("Server state downloaded successfully!");
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Download error: ${err}`);
        });
    }, [gameId, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleSyncSaves = useCallback(() => {
        return syncFiles('saves', saves, serverSaves, UploadSave, DownloadServerSave);
    }, [syncFiles, saves, serverSaves]);

    const handleSyncStates = useCallback(() => {
        return syncFiles('states', states, serverStates, UploadState, DownloadServerState);
    }, [syncFiles, states, serverStates]);

    const handleSmartSync = useCallback(async () => {
        if (offlineMode) return;
        setDownloadStatus("Starting full smart sync...");
        await handleSyncSaves();
        await handleSyncStates();
        setSuccessStatus("Smart sync complete!");
    }, [offlineMode, handleSyncSaves, handleSyncStates, setDownloadStatus, setSuccessStatus]);

    const hasSavesOrStates = serverSaves.length > 0 || saves.length > 0 || serverStates.length > 0 || states.length > 0;

    return {
        saves,
        states,
        serverSaves,
        serverStates,
        hasSavesOrStates,
        fetchAppData,
        handleDeleteSave,
        handleDeleteState,
        handleUploadSave,
        handleUploadState,
        handleDownloadServerSave,
        handleDownloadServerState,
        handleSmartSync,
    };
}

export function GamePage({ gameId, onBack }: GamePageProps) {
    const [game, setGame] = useState<types.Game | null>(null);
    const [loading, setLoading] = useState(true);
    const [downloading, setDownloading] = useState(false);
    const [downloadStatus, setDownloadStatus] = useState<string | null>(null);
    const [isDownloaded, setIsDownloaded] = useState(false);
    const [statusChecked, setStatusChecked] = useState(false);
    const [downloadProgress, setDownloadProgress] = useState<number>(0);
    const [statusFading, setStatusFading] = useState(false);
    const [isPlaying, setIsPlaying] = useState(false);
    const [isExtracting, setIsExtracting] = useState(false);
    const [availableCores, setAvailableCores] = useState<string[]>([]);
    const [selectedCore, setSelectedCore] = useState<string>('');
    const [isPickerOpen, setIsPickerOpen] = useState(false);
    const [firmwares, setFirmwares] = useState<types.Firmware[]>([]);
    const [selectedFirmwareId, setSelectedFirmwareId] = useState<number>(0);
    const [isFirmwarePickerOpen, setIsFirmwarePickerOpen] = useState(false);
    const [offlineMode, setOfflineMode] = useState(false);

    const fadeTimeoutRef = useRef<any>(null);
    const clearStatusTimeoutRef = useRef<any>(null);
    const statusSequenceRef = useRef(0);

    const setSuccessStatus = (msg: string) => {
        const sequence = ++statusSequenceRef.current;

        if (fadeTimeoutRef.current) clearTimeout(fadeTimeoutRef.current);
        if (clearStatusTimeoutRef.current) clearTimeout(clearStatusTimeoutRef.current);

        setStatusFading(false);
        setDownloadStatus(msg);

        fadeTimeoutRef.current = setTimeout(() => {
            if (statusSequenceRef.current === sequence) {
                setStatusFading(true);
            }
        }, 1000);

        clearStatusTimeoutRef.current = setTimeout(() => {
            if (statusSequenceRef.current === sequence) {
                setDownloadStatus(prev => prev === msg ? null : prev);
                setStatusFading(false);
            }
        }, 3000);
    };

    const {
        saves,
        states,
        serverSaves,
        serverStates,
        hasSavesOrStates,
        fetchAppData,
        handleDeleteSave,
        handleDeleteState,
        handleUploadSave,
        handleUploadState,
        handleDownloadServerSave,
        handleDownloadServerState,
        handleSmartSync,
    } = useGameSavesAndStates(
        gameId,
        offlineMode,
        isDownloaded,
        setDownloadStatus,
        setSuccessStatus
    );

    const [firmwareDownloading, setFirmwareDownloading] = useState(false);
    const [firmwareStatus, setFirmwareStatus] = useState<string>('');

    const focusFirstAvailableSaveState = () => {
        if (serverSaves.length > 0) {
            setFocus('server-save-0-download');
        } else if (saves.length > 0) {
            setFocus('save-0-upload');
        } else if (serverStates.length > 0) {
            setFocus('server-state-0-download');
        } else if (states.length > 0) {
            setFocus('state-0-upload');
        }
    };

    useEffect(() => {
        const unsubscribe = EventsOn("offline-mode-changed", (newOfflineMode: boolean) => {
            setOfflineMode(newOfflineMode);
        });
        return () => unsubscribe();
    }, []);

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

        const unlistenStatus = EventsOn("library-status", (data: { game_id: number; status: string }) => {
            if (data.game_id === gameId && data.status === "extracting") {
                setIsExtracting(true);
                setDownloadStatus("Extracting files...");
            }
        });

        const unlistenStarted = EventsOn(APP_EVENTS.GAME_STARTED, () => setIsPlaying(true));
        const unlistenExited = EventsOn(APP_EVENTS.GAME_EXITED, () => setIsPlaying(false));

        return () => {
            unlisten();
            unlistenStatus();
            unlistenStarted();
            unlistenExited();
        };
    }, [gameId]);

    const { ref } = useFocusable({
        onArrowPress: (direction: string) => {
            return true;
        },
    });

    useEffect(() => {
        if (gameId) {
            GetCoresForGame(gameId).then((cores: string[]) => {
                setAvailableCores(cores || []);
                if (cores && cores.length > 0) setSelectedCore(cores[0]);
            }).catch((err: any) => {
                console.warn('GetCoresForGame failed:', err);
            });
        }
    }, [gameId]);

    const closePicker = useCallback(() => {
        setIsPickerOpen(false);
        setTimeout(() => setFocus('core-selector'), 100);
    }, []);

    const closeFirmwarePicker = useCallback(() => {
        setIsFirmwarePickerOpen(false);
        setTimeout(() => setFocus('firmware-selector'), 100);
    }, []);

    // Download Handler
    const handleDownload = useCallback(() => {
        if (!game || downloading || isDownloaded) return;
        setDownloading(true);
        setDownloadStatus("Downloading...");
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
                setIsExtracting(false);
            });
    }, [game, downloading, isDownloaded]);

    const handleCancel = useCallback(() => {
        if (!game) return;
        CancelDownload(game.id);
        setDownloadStatus("Cancellation requested...");
    }, [game]);

    // Play Handler
    const handlePlay = useCallback(() => {
        if (!game || isPlaying) return;
        setDownloadStatus("Starting RetroArch...");
        PlayRomWithCore(game.id, selectedCore).then(() => {
            setSuccessStatus("Game launched successfully!");
        }).catch((err: string) => {
            if (err.includes("launch cancelled")) {
                setDownloadStatus("");
            } else {
                setDownloadStatus(`Play error: ${err}`);
            }
        });
    }, [game, isPlaying, selectedCore]);

    // Open Folder Handler
    const handleOpenFolder = useCallback(() => {
        if (!game) return;
        OpenGameFolder(game).catch((err: string) => {
            setDownloadStatus(`Open folder error: ${err}`);
        });
    }, [game]);

    // Delete Handler
    const handleDelete = useCallback(() => {
        if (!game || isPlaying) return;
        DeleteRom(game.id).then(() => {
            setIsDownloaded(false);
            setSuccessStatus("ROM deleted from library.");
            setTimeout(() => setFocus('download-button'), 100);
        }).catch((err: string) => {
            setDownloadStatus(`Delete error: ${err}`);
        });
    }, [game, isPlaying]);

    useEffect(() => {
        GetRom(gameId)
            .then((res: types.Game) => {
                setGame(res);

                // Fetch firmwares for this platform
                GetFirmware(res.platform_id).then(list => {
                    setFirmwares(list || []);

                    // Get current config to see if a firmware is already selected
                    GetConfig().then(cfg => {
                        if (cfg.platform_firmware && cfg.platform_firmware[res.platform_slug]) {
                            setSelectedFirmwareId(cfg.platform_firmware[res.platform_slug]);
                        }
                        setOfflineMode(cfg.offline_mode || false);
                    });
                }).catch(err => console.error("Failed to fetch firmwares:", err));

                // Check if already downloaded
                GetRomDownloadStatus(gameId).then((status: boolean) => {
                    setIsDownloaded(status);
                    setStatusChecked(true);
                    // Set focus to the primary button after data is loaded
                    setTimeout(() => {
                        if (status) {
                            if (availableCores.length > 1) {
                                // If multiple cores, focus the selector first so user can choose
                                setFocus('core-selector');
                            } else {
                                setFocus('play-button');
                            }
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

    const handleFirmwareChange = async (id: number) => {
        if (!game) return;
        setSelectedFirmwareId(id);

        const fw = getTargetFirmware(firmwares, id);
        if (!fw) return;

        setFirmwareDownloading(true);
        setFirmwareStatus('Downloading...');

        try {
            await SetPlatformFirmware(game.platform_slug, fw);
            setFirmwareStatus('Downloaded');
            closeFirmwarePicker();
            setTimeout(() => setFirmwareStatus(''), 3000);
        } catch (err) {
            console.error("Failed to set firmware:", err);
            setFirmwareStatus('Error');
        } finally {
            setFirmwareDownloading(false);
        }
    };

    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            const activeElement = document.activeElement;
            const isTyping = activeElement?.tagName === 'INPUT' || activeElement?.tagName === 'TEXTAREA';
            if (isTyping) return;

            if (e.key.toLowerCase() === 'r') {
                handleSmartSync();
            }

            handleEscapeKey(e, isPickerOpen, isFirmwarePickerOpen, closePicker, closeFirmwarePicker);
        };

        window.addEventListener('keydown', handleKeyDown, true);
        return () => window.removeEventListener('keydown', handleKeyDown, true);
    }, [isPickerOpen, isFirmwarePickerOpen, closePicker, closeFirmwarePicker, handleSmartSync]);


    return (
        <div id="game-page" ref={ref}>
            {loading ? (
                <div className="game-page-loading">Loading game details...</div>
            ) : !game ? (
                <div className="game-page-error">Game not found.</div>
            ) : (
                <>
                    <div className="library-header-extras" style={{ top: '5rem', right: '11rem' }}>
                        {offlineMode && (
                            <div className="offline-badge">
                                Offline Mode
                            </div>
                        )}
                    </div>
                    {isPickerOpen && (
                        <div className="core-picker-overlay" onClick={closePicker}>
                            <div className="core-picker-modal" onClick={e => e.stopPropagation()}>
                                <div className="core-picker-header">
                                    <h3>Select Core</h3>
                                </div>
                                <div className="core-picker-list">
                                    {availableCores.map((core, idx) => (
                                        <PickerOption
                                            key={core}
                                            name={core.replace('_libretro', '').replace(/_/g, ' ')}
                                            isSelected={core === selectedCore}
                                            isFirst={idx === 0}
                                            onSelect={() => {
                                                setSelectedCore(core);
                                                closePicker();
                                            }}
                                            focusKey={`core-option-${idx}`}
                                            className="core-option"
                                        />
                                    ))}
                                </div>
                                <CancelButton onCancel={closePicker} />
                            </div>
                        </div>
                    )}
                    {isFirmwarePickerOpen && (
                        <div className="core-picker-overlay" onClick={closeFirmwarePicker}>
                            <div className="core-picker-modal" onClick={e => e.stopPropagation()}>
                                <div className="core-picker-header">
                                    <h3>Select Firmware</h3>
                                </div>
                                <div className="core-picker-list">
                                    <PickerOption
                                        name="No Firmware"
                                        isSelected={selectedFirmwareId === 0}
                                        isFirst={true}
                                        onSelect={() => handleFirmwareChange(0)}
                                        focusKey="firmware-option-0"
                                        className="firmware-option"
                                    />
                                    {firmwares.map((fw, idx) => (
                                        <PickerOption
                                            key={fw.id}
                                            name={`${fw.file_name} ${fw.is_verified ? '✓' : ''}`}
                                            isSelected={fw.id === selectedFirmwareId}
                                            isFirst={false}
                                            onSelect={() => handleFirmwareChange(fw.id)}
                                            focusKey={`firmware-option-${idx + 1}`}
                                            className="firmware-option"
                                        />
                                    ))}
                                </div>
                                <CancelButton onCancel={closeFirmwarePicker} />
                            </div>
                        </div>
                    )}
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
                                    !offlineMode ? (
                                        <InnerDownloadButton
                                            isDisabled={downloading || isPlaying}
                                            isDownloading={downloading}
                                            isExtracting={isExtracting}
                                            hasSaves={hasSavesOrStates}
                                            onDownload={handleDownload}
                                            onCancel={handleCancel}
                                            onFocusSaves={focusFirstAvailableSaveState}
                                        />
                                    ) : (
                                        <div className="offline-notice">
                                            Download unavailable in offline mode
                                        </div>
                                    )
                                ) : (
                                    <div className="game-actions-vertical">
                                        <div className="game-firmware-section">
                                            <h3>Platform Firmware</h3>
                                            {firmwares.length > 0 ? (
                                                <InnerFirmwareSelector
                                                    firmwares={firmwares}
                                                    selectedId={selectedFirmwareId}
                                                    isDownloading={firmwareDownloading}
                                                    status={firmwareStatus}
                                                    hasSaves={hasSavesOrStates}
                                                    onClick={() => setIsFirmwarePickerOpen(true)}
                                                    onFocusRequest={() => setFocus('firmware-selector')}
                                                    onFocusSaves={focusFirstAvailableSaveState}
                                                />
                                            ) : (
                                                <div className="firmware-status">No firmware available in RomM</div>
                                            )}
                                        </div>
                                        <div className="game-core-section">
                                            <h3>Core</h3>
                                            {availableCores.length > 0 && (
                                                <InnerCoreSelector
                                                    currentCore={selectedCore}
                                                    isDisabled={isPlaying}
                                                    hasFirmware={firmwares.length > 0}
                                                    hasSaves={hasSavesOrStates}
                                                    onClick={() => setIsPickerOpen(true)}
                                                    onFocusRequest={() => setFocus('core-selector')}
                                                    onFocusSaves={focusFirstAvailableSaveState}
                                                />
                                            )}
                                        </div>
                                        <InnerPlayButton
                                            isDisabled={isPlaying}
                                            hasCore={availableCores.length > 0}
                                            hasFirmware={firmwares.length > 0}
                                            hasSaves={hasSavesOrStates}
                                            onPlay={handlePlay}
                                            onFocusSaves={focusFirstAvailableSaveState}
                                        />
                                        <InnerOpenFolderButton
                                            hasSaves={hasSavesOrStates}
                                            onOpenFolder={handleOpenFolder}
                                            onFocusSaves={focusFirstAvailableSaveState}
                                        />
                                        <InnerDeleteButton
                                            isDisabled={isPlaying}
                                            hasSaves={hasSavesOrStates}
                                            onDelete={handleDelete}
                                            onFocusDownload={() => setFocus('download-button')}
                                            onFocusSaves={focusFirstAvailableSaveState}
                                        />
                                    </div>
                                )
                            )}
                            <div className={`status-display ${statusFading ? 'fading' : ''}`}>
                                {downloadStatus}
                                {downloading && (
                                    <div className="progress-wrapper">
                                        <div className="progress-container">
                                            <div className="progress-bar" style={{ width: `${downloadProgress}%` }}></div>
                                        </div>
                                        <span className="progress-percentage">{Math.round(downloadProgress)}%</span>
                                    </div>
                                )}
                            </div>
                        </div>

                        <div className="game-main-info">
                            <div className="game-header-row">
                                <h1 className="game-title">{decodeHtml(game.name)}</h1>
                            </div>
                            <div className="game-meta">
                                {game.genres && game.genres.length > 0 && (
                                    <div className="game-genres">
                                        {game.genres.map((genre: string, idx: number) => (
                                            <span key={idx} className="genre-tag">{genre}</span>
                                        ))}
                                    </div>
                                )}
                            </div>
                            <div className="game-description">
                                <h3>Summary</h3>
                                <p>
                                    {decodeHtml(game.summary || "No description available.")}
                                </p>
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
                                                isDisabled={isPlaying || offlineMode}
                                            />
                                        ))}
                                        {(serverSaves.length === 0 || offlineMode) && <p className="no-files">{offlineMode ? "Server sync unavailable offline" : "No server saves found."}</p>}
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
                                                isOffline={offlineMode}
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
                                                isDisabled={isPlaying || offlineMode}
                                            />
                                        ))}
                                        {(serverStates.length === 0 || offlineMode) && <p className="no-files">{offlineMode ? "Server sync unavailable offline" : "No server states found."}</p>}
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
                                                isOffline={offlineMode}
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
                            <LegendItem buttonAction="west" keyLabel="R" label="Sync" />
                            <LegendItem buttonAction="east" keyLabel="ESC" label="Back" />
                            <LegendItem buttonAction="south" keyLabel="ENTER" label="OK" />
                        </div>
                    </div>
                </>
            )}
        </div>
    );
}

const handleDownloadButtonArrowPress = (direction: string, hasSaves: boolean, onFocusSaves: () => void) => {
    if (direction === 'left') return false;
    if (direction === 'right') {
        if (hasSaves) onFocusSaves();
        return false;
    }
    return direction === 'down';
};

const handleDownloadButtonMouseEnter = (isDownloading: boolean, isDisabled: boolean) => {
    if (!getMouseActive()) return;
    if (isDownloading || !isDisabled) {
        setFocus('download-button');
    }
};

const getDownloadBtnClassName = (focused: boolean, isBtnDisabled: boolean, isDownloading: boolean) => {
    let className = "btn download-btn";
    if (focused) className += " focused";
    if (isBtnDisabled) className += " disabled";
    if (isDownloading) className += " cancel-mode";
    return className;
};

const getDownloadBtnIconAndText = (isDownloading: boolean, isExtracting: boolean) => {
    const icon = isDownloading ? <TrashIcon /> : <DownloadIcon />;
    let text = "Download to Library";
    if (isDownloading) text = "Cancel Download";
    if (isExtracting) text = "Extracting...";
    return { icon, text };
};

function InnerDownloadButton({ isDisabled, isDownloading, isExtracting, hasSaves, onDownload, onCancel, onFocusSaves }: {
    isDisabled: boolean;
    isDownloading: boolean;
    isExtracting: boolean;
    hasSaves: boolean;
    onDownload: () => void;
    onCancel: () => void;
    onFocusSaves: () => void;
}) {
    const handleAction = isDownloading ? onCancel : onDownload;

    const { ref, focused } = useFocusable({
        focusKey: 'download-button',
        onArrowPress: (direction: string) => handleDownloadButtonArrowPress(direction, hasSaves, onFocusSaves),
        onEnterPress: handleAction
    });

    const isBtnDisabled = isDisabled && !isDownloading;
    const btnClassName = getDownloadBtnClassName(focused, isBtnDisabled, isDownloading);
    const { icon, text } = getDownloadBtnIconAndText(isDownloading, isExtracting);

    return (
        <button
            ref={ref}
            className={btnClassName}
            disabled={isBtnDisabled}
            onMouseEnter={() => handleDownloadButtonMouseEnter(isDownloading, isDisabled)}
            onClick={handleAction}
        >
            <div className="btn-content">
                {icon}
                <span>{text}</span>
            </div>
        </button>
    );
}

function InnerCoreSelector({ currentCore, isDisabled, hasFirmware, hasSaves, onClick, onFocusRequest, onFocusSaves }: {
    currentCore: string;
    isDisabled: boolean;
    hasFirmware: boolean;
    hasSaves: boolean;
    onClick: () => void;
    onFocusRequest: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'core-selector',
        onArrowPress: (direction: string) => {
            switch (direction) {
                case 'up':
                    if (hasFirmware) setFocus('firmware-selector');
                    return false;
                case 'down':
                    setFocus('play-button');
                    return false;
                case 'right':
                    hasSaves ? onFocusSaves() : setFocus('play-button');
                    return false;
                case 'left':
                    return false;
                default:
                    return true;
            }
        },
        onEnterPress: onClick
    });

    return (
        <div
            id="core-select"
            ref={ref}
            className={`core-selector-button ${focused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
            onMouseEnter={() => {
                if (getMouseActive() && !isDisabled) {
                    onFocusRequest();
                }
            }}
            onClick={onClick}
        >
            <span className="current-core">
                {currentCore.replace('_libretro', '').replace(/_/g, ' ')}
            </span>
            <div className="dropdown-arrow"></div>
        </div>
    );
}

function InnerPlayButton({ isDisabled, hasCore, hasFirmware, hasSaves, onPlay, onFocusSaves }: {
    isDisabled: boolean;
    hasCore: boolean;
    hasFirmware: boolean;
    hasSaves: boolean;
    onPlay: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'play-button',
        onArrowPress: (direction: string) => {
            switch (direction) {
                case 'up':
                    if (hasCore) setFocus('core-selector');
                    else if (hasFirmware) setFocus('firmware-selector');
                    return false;
                case 'down':
                    setFocus('open-folder-button');
                    return false;
                case 'right':
                    if (hasSaves) onFocusSaves();
                    return false;
                case 'left':
                    return false;
                default:
                    return true;
            }
        },
        onEnterPress: onPlay
    });

    return (
        <button
            ref={ref}
            className={`btn play-btn ${focused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
            disabled={isDisabled}
            onMouseEnter={() => {
                if (getMouseActive() && !isDisabled) {
                    setFocus('play-button');
                }
            }}
            onClick={onPlay}
        >
            <div className="btn-content">
                <PlayIcon />
                <span>Play</span>
            </div>
        </button>
    );
}

function InnerOpenFolderButton({ hasSaves, onOpenFolder, onFocusSaves }: {
    hasSaves: boolean;
    onOpenFolder: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'open-folder-button',
        onArrowPress: (direction: string) => {
            if (direction === 'up') {
                setFocus('play-button');
                return false;
            }
            if (direction === 'down') {
                setFocus('delete-button');
                return false;
            }
            if (direction === 'right' && hasSaves) {
                onFocusSaves();
                return false;
            }
            if (direction === 'left') return false;
            return true;
        },
        onEnterPress: onOpenFolder
    });

    return (
        <button
            ref={ref}
            className={`btn open-folder-btn ${focused ? 'focused' : ''}`}
            onMouseEnter={() => {
                if (getMouseActive()) {
                    setFocus('open-folder-button');
                }
            }}
            onClick={onOpenFolder}
        >
            <div className="btn-content">
                <FolderIcon size={20} />
                <span>Open Folder</span>
            </div>
        </button>
    );
}

function InnerDeleteButton({ isDisabled, hasSaves, onDelete, onFocusDownload, onFocusSaves }: {
    isDisabled: boolean;
    hasSaves: boolean;
    onDelete: () => void;
    onFocusDownload: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'delete-button',
        onArrowPress: (direction: string) => {
            if (direction === 'up') {
                setFocus('open-folder-button');
                return false;
            }
            if (direction === 'right' && hasSaves) {
                onFocusSaves();
                return false;
            }
            if (direction === 'left') return false;
            return true;
        },
        onEnterPress: onDelete
    });

    return (
        <button
            ref={ref}
            className={`btn delete-btn ${focused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
            disabled={isDisabled}
            title="Delete ROM"
            onMouseEnter={() => {
                if (getMouseActive() && !isDisabled) {
                    setFocus('delete-button');
                }
            }}
            onClick={onDelete}
        >
            <div className="btn-content">
                <TrashIcon />
                <span>Delete</span>
            </div>
        </button>
    );
}

interface PickerOptionProps {
    name: string;
    isSelected: boolean;
    onSelect: () => void;
    focusKey: string;
    isFirst: boolean;
    className: string;
}

function PickerOption({ name, isSelected, onSelect, focusKey, isFirst, className }: PickerOptionProps) {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress: onSelect,
        onArrowPress: (direction: string) => {
            if (direction === 'left' || direction === 'right') return false;
            if (direction === 'up' && isFirst) return false;
            return true;
        }
    });

    useEffect(() => {
        if (isSelected) {
            setFocus(focusKey);
        }
    }, [isSelected, focusKey]);

    return (
        <div
            ref={ref}
            className={`${className} ${focused ? 'focused' : ''} ${isSelected ? 'selected' : ''}`}
            onClick={onSelect}
            onMouseEnter={() => {
                if (getMouseActive()) {
                    setFocus(focusKey);
                }
            }}
        >
            <span className={`${className}-name`}>{name}</span>
            {isSelected && <span className="selected-check">✓</span>}
        </div>
    );
}

function CancelButton({ onCancel }: { onCancel: () => void }) {
    const { ref, focused } = useFocusable({
        focusKey: 'picker-cancel',
        onEnterPress: onCancel,
        onArrowPress: (direction: string) => {
            // Block left/right/down
            if (direction === 'left' || direction === 'right' || direction === 'down') return false;
            return true;
        }
    });

    return (
        <button
            ref={ref}
            className={`btn cancel-btn ${focused ? 'focused' : ''}`}
            onClick={onCancel}
            onMouseEnter={() => {
                if (getMouseActive()) {
                    setFocus('picker-cancel');
                }
            }}
        >
            Cancel
        </button>
    );
}

const getFirmwareDisplayText = (selectedId: number, selectedFw?: types.Firmware) => {
    if (selectedId === 0) return "No Firmware";
    return selectedFw?.file_name || "Unknown Firmware";
};

function InnerFirmwareSelector({ firmwares, selectedId, isDownloading, status, hasSaves, onClick, onFocusRequest, onFocusSaves }: {
    firmwares: types.Firmware[];
    selectedId: number;
    isDownloading: boolean;
    status: string;
    hasSaves: boolean;
    onClick: () => void;
    onFocusRequest: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'firmware-selector',
        onArrowPress: (direction: string) => {
            switch (direction) {
                case 'up':
                case 'left':
                    return false;
                case 'down':
                    setFocus('core-selector');
                    return false;
                case 'right':
                    if (hasSaves) onFocusSaves();
                    return false;
                default:
                    return true;
            }
        },
        onEnterPress: onClick
    });

    const selectedFw = firmwares.find(f => f.id === selectedId);
    const displayText = getFirmwareDisplayText(selectedId, selectedFw);

    return (
        <div className="platform-firmware-selector">
            <div
                ref={ref}
                className={`firmware-selector-button ${focused ? 'focused' : ''} ${isDownloading ? 'disabled' : ''}`}
                onMouseEnter={() => {
                    if (getMouseActive() && !isDownloading) {
                        onFocusRequest();
                    }
                }}
                onClick={onClick}
            >
                <span className="current-firmware">
                    {displayText}
                </span>
                <div className="dropdown-arrow"></div>
            </div>
            {status && (
                <div className={`firmware-status ${status.toLowerCase()}`}>
                    {status}
                </div>
            )}
        </div>
    );
}

// FirmwareOption deleted, PickerOption used instead
