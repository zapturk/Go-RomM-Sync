import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { GetRom, DownloadRomToLibrary, GetRomDownloadStatus, DeleteRom, PlayRomWithCore, GetCoresForGame,
    GetSaves, GetStates, DeleteSave, DeleteState, UploadSave, UploadState,
    GetServerSaves, GetServerStates, DownloadServerSave, DownloadServerState,
    OpenGameFolder, GetFirmware, SetPlatformFirmware, GetConfig, CancelDownload,
    GetGameController, SetGameController,
    GetRomStartupFiles, GetGameStartupFile, SetGameStartupFile,
    GetSaveSlots, GetGameSaveSlot, SetGameSaveSlot, CreateSaveSlot, DeleteSaveSlot, DeleteServerSave, UploadSaveToSlot,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { GameCover } from "./GameCover";
import { TrashIcon, FolderIcon, PlayIcon, DownloadIcon } from "./components/Icons";
import { FileItemRow, getItemName, getItemCore } from "./FileItemRow";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { TIMESTAMP_REGEX, APP_EVENTS, WII_CONTROLLER_OPTIONS } from './constants';
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



const getCleanName = (item: any) => {
    const raw = item?.name || item?.file_name || '';
    return raw.replace(TIMESTAMP_REGEX, '');
};

const isMatchingItem = (o: any, name: string, core: string) => 
    getCleanName(o) === name && getItemCore(o) === core;

const getItemTime = (item: any): number => {
    if (!item || !item.updated_at) return 0;
    return new Date(item.updated_at).getTime();
};

const getFileStatus = (item: any, otherList: any[], isSave = true) => {
    const cleanName = getCleanName(item);
    const core = getItemCore(item);
    const itemTime = getItemTime(item);
    if (!cleanName || !itemTime) return undefined;

    let other = otherList.find(o => isMatchingItem(o, cleanName, core));
    if (!other && isSave) {
        other = otherList.find(o => getCleanName(o) === cleanName);
    }
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
    isControllerPickerOpen: boolean,
    isStartupFilePickerOpen: boolean,
    isNewSlotModalOpen: boolean,
    isDeleteSlotModalOpen: boolean,
    closePicker: () => void,
    closeFirmwarePicker: () => void,
    closeControllerPicker: () => void,
    closeStartupFilePicker: () => void,
    closeNewSlotModal: () => void,
    closeDeleteSlotModal: () => void
): boolean => {
    if (e.key !== 'Escape') return false;
    if (isNewSlotModalOpen) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closeNewSlotModal();
        return true;
    }
    if (isDeleteSlotModalOpen) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closeDeleteSlotModal();
        return true;
    }
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
    if (isControllerPickerOpen) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closeControllerPicker();
        return true;
    }
    if (isStartupFilePickerOpen) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closeStartupFilePicker();
        return true;
    }
    return false;
};

function useGameSavesAndStates(
    gameId: number,
    offlineMode: boolean,
    isDownloaded: boolean,
    setDownloadStatus: (status: string | null) => void,
    setSuccessStatus: (msg: string) => void,
    selectedCore?: string
) {
    const [saves, setSaves] = useState<types.FileItem[]>([]);
    const [states, setStates] = useState<types.FileItem[]>([]);
    const [serverSaves, setServerSaves] = useState<types.ServerSave[]>([]);
    const [serverStates, setServerStates] = useState<types.ServerState[]>([]);
    const [slots, setSlots] = useState<types.SaveSlot[]>([]);
    const [activeSlot, setActiveSlot] = useState<string>('default');

    const fetchAppData = useCallback(() => {
        GetSaves(gameId).then(res => setSaves(res || [])).catch(console.error);
        GetStates(gameId).then(res => setStates(res || [])).catch(console.error);
        GetServerSaves(gameId).then(res => setServerSaves(res || [])).catch(console.error);
        GetServerStates(gameId).then(res => setServerStates(res || [])).catch(console.error);
        GetSaveSlots(gameId).then(res => setSlots(res || [])).catch(console.error);
        GetGameSaveSlot(gameId).then(res => setActiveSlot(res || 'default')).catch(console.error);
    }, [gameId]);

    const filteredServerSaves = useMemo(() => {
        const slotSaves = serverSaves.filter(s => {
            const sSlot = s.slot || '';
            if (activeSlot === '') {
                return sSlot === '';
            }
            return sSlot === activeSlot;
        });

        // Group by clean file name, picking the most recent (highest updated_at, tie-break id)
        const latestByName = new Map<string, types.ServerSave>();
        for (const save of slotSaves) {
            const clean = save.file_name.replace(TIMESTAMP_REGEX, "");
            const existing = latestByName.get(clean);
            if (!existing) {
                latestByName.set(clean, save);
            } else {
                const saveTime = new Date(save.updated_at).getTime();
                const existingTime = new Date(existing.updated_at).getTime();
                if (saveTime > existingTime || (saveTime === existingTime && save.id > existing.id)) {
                    latestByName.set(clean, save);
                }
            }
        }

        return Array.from(latestByName.values()).sort((a, b) => {
            const diff = new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
            return diff !== 0 ? diff : b.id - a.id;
        });
    }, [serverSaves, activeSlot]);

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
            const matchingServerSaves = serverList.filter(s => s.file_name.replace(TIMESTAMP_REGEX, "") === name);
            const serverClean = matchingServerSaves.sort((a, b) => {
                const diff = new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
                return diff !== 0 ? diff : b.id - a.id;
            })[0];

            const action = determineSyncAction(local, serverClean);
            if (action === SyncAction.Upload && local) {
                await uploadFn(gameId, local.core, local.name).catch(console.error);
            } else if (action === SyncAction.Download && serverClean) {
                const targetEmulator = (type === 'saves' && selectedCore) ? selectedCore : serverClean.emulator;
                await downloadFn(gameId, serverClean.id, targetEmulator, name, serverClean.updated_at).catch(console.error);
            }
        }
        setSuccessStatus(`Smart sync for ${type} complete!`);
        fetchAppData();
    }, [gameId, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleSelectSlot = useCallback((slotName: string) => {
        SetGameSaveSlot(gameId, slotName).then(() => {
            setActiveSlot(slotName);
            setSuccessStatus(`Switched to save slot: ${slotName || 'Legacy'}`);
            GetSaveSlots(gameId).then(res => setSlots(res || [])).catch(console.error);
        }).catch((err: string) => {
            setDownloadStatus(`Error selecting slot: ${err}`);
        });
    }, [gameId, setSuccessStatus, setDownloadStatus]);

    const handleCreateSlot = useCallback((slotName: string) => {
        const trimmed = slotName.trim();
        if (!trimmed) {
            setDownloadStatus("Slot name cannot be empty");
            return;
        }
        CreateSaveSlot(gameId, trimmed).then(async () => {
            setActiveSlot(trimmed);
            if (saves.length > 0) {
                setDownloadStatus(`Backfilling local saves to slot "${trimmed}"...`);
                for (const s of saves) {
                    try {
                        await UploadSaveToSlot(gameId, s.core, s.name, trimmed);
                    } catch (e) {
                        console.error("Backfill upload error:", e);
                    }
                }
                setSuccessStatus(`Created save slot "${trimmed}" and backfilled local save!`);
            } else {
                setSuccessStatus(`Created save slot: ${trimmed}`);
            }
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Error creating slot: ${err}`);
        });
    }, [gameId, saves, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const handleDeleteSlot = useCallback((slotName: string) => {
        const trimmed = slotName.trim();
        if (!trimmed) {
            setDownloadStatus("Legacy slot cannot be deleted");
            return;
        }
        DeleteSaveSlot(gameId, trimmed).then(() => {
            setSuccessStatus(`Deleted save slot: ${trimmed}`);
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Error deleting slot: ${err}`);
        });
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

    const handleDeleteServerSave = useCallback((saveId: number) => {
        DeleteServerSave(saveId).then(() => {
            setSuccessStatus("Server save deleted from RomM.");
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Error deleting server save: ${err}`);
        });
    }, [fetchAppData, setSuccessStatus, setDownloadStatus]);

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
        const targetEmulator = selectedCore || save.emulator;
        DownloadServerSave(gameId, save.id, targetEmulator, cleanFileName, save.updated_at).then(() => {
            setSuccessStatus("Server save downloaded successfully!");
            fetchAppData();
        }).catch((err: string) => {
            setDownloadStatus(`Download error: ${err}`);
        });
    }, [gameId, selectedCore, fetchAppData, setSuccessStatus, setDownloadStatus]);

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
        return syncFiles('saves', saves, filteredServerSaves, UploadSave, DownloadServerSave);
    }, [syncFiles, saves, filteredServerSaves]);

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

    const handleBackfillCurrentSlot = useCallback(async () => {
        if (offlineMode || saves.length === 0) return;
        setDownloadStatus(`Backfilling slot "${activeSlot || 'Legacy'}" with local saves...`);
        for (const s of saves) {
            try {
                await UploadSaveToSlot(gameId, s.core, s.name, activeSlot);
            } catch (e) {
                console.error("Backfill error:", e);
            }
        }
        setSuccessStatus(`Successfully backfilled slot "${activeSlot || 'Legacy'}"!`);
        fetchAppData();
    }, [gameId, saves, activeSlot, offlineMode, fetchAppData, setSuccessStatus, setDownloadStatus]);

    const hasSavesOrStates = serverSaves.length > 0 || saves.length > 0 || serverStates.length > 0 || states.length > 0;

    return {
        saves,
        states,
        serverSaves,
        filteredServerSaves,
        serverStates,
        slots,
        activeSlot,
        hasSavesOrStates,
        fetchAppData,
        handleSelectSlot,
        handleCreateSlot,
        handleDeleteSlot,
        handleDeleteSave,
        handleDeleteServerSave,
        handleDeleteState,
        handleUploadSave,
        handleUploadState,
        handleDownloadServerSave,
        handleDownloadServerState,
        handleSmartSync,
        handleBackfillCurrentSlot,
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
    const [selectedControllerType, setSelectedControllerType] = useState<string>('769');
    const [isControllerPickerOpen, setIsControllerPickerOpen] = useState(false);
    const [startupFiles, setStartupFiles] = useState<string[]>([]);
    const [selectedStartupFile, setSelectedStartupFile] = useState<string>('');
    const [isStartupFilePickerOpen, setIsStartupFilePickerOpen] = useState(false);
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
        filteredServerSaves,
        serverStates,
        slots,
        activeSlot,
        hasSavesOrStates,
        fetchAppData,
        handleSelectSlot,
        handleCreateSlot,
        handleDeleteSlot,
        handleDeleteSave,
        handleDeleteServerSave,
        handleDeleteState,
        handleUploadSave,
        handleUploadState,
        handleDownloadServerSave,
        handleDownloadServerState,
        handleSmartSync,
        handleBackfillCurrentSlot,
    } = useGameSavesAndStates(
        gameId,
        offlineMode,
        isDownloaded,
        setDownloadStatus,
        setSuccessStatus,
        selectedCore
    );

    const [isNewSlotModalOpen, setIsNewSlotModalOpen] = useState(false);
    const [isDeleteSlotModalOpen, setIsDeleteSlotModalOpen] = useState(false);
    const [newSlotName, setNewSlotName] = useState('');
    const newSlotInputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        if (isNewSlotModalOpen) {
            setTimeout(() => newSlotInputRef.current?.focus(), 50);
        }
    }, [isNewSlotModalOpen]);

    const handleCreateSlotSubmit = () => {
        const trimmed = newSlotName.trim();
        if (!trimmed) return;
        handleCreateSlot(trimmed);
        setIsNewSlotModalOpen(false);
        setNewSlotName('');
    };

    const handleDeleteSlotSubmit = () => {
        if (!activeSlot) return;
        handleDeleteSlot(activeSlot);
        setIsDeleteSlotModalOpen(false);
    };

    const [firmwareDownloading, setFirmwareDownloading] = useState(false);
    const [firmwareStatus, setFirmwareStatus] = useState<string>('');

    const focusFirstAvailableSaveState = () => {
        if (slots.length > 0) {
            setFocus('slot-pill-0');
        } else if (filteredServerSaves.length > 0) {
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
        const unlistenExited = EventsOn(APP_EVENTS.GAME_EXITED, () => {
            setIsPlaying(false);
            fetchAppData();
        });

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
            GetGameController(gameId).then((ctrl: string) => {
                if (ctrl) setSelectedControllerType(ctrl);
            }).catch((err: any) => {
                console.warn('GetGameController failed:', err);
            });
            GetRomStartupFiles(gameId).then((files: string[]) => {
                const list = files || [];
                setStartupFiles(list);
                GetGameStartupFile(gameId).then((saved: string) => {
                    if (saved && list.includes(saved)) {
                        setSelectedStartupFile(saved);
                    } else if (list.length > 0) {
                        setSelectedStartupFile(list[0]);
                    }
                }).catch(() => {
                    if (list.length > 0) setSelectedStartupFile(list[0]);
                });
            }).catch((err: any) => {
                console.warn('GetRomStartupFiles failed:', err);
            });
        }
    }, [gameId]);

    const platformSlug = (game?.platform_slug || game?.platform?.slug || '').toLowerCase();
    const platformName = (game?.platform_display_name || game?.platform?.name || '').toLowerCase();
    const fullPath = (game?.full_path || '').toLowerCase();
    const isGameCube = platformSlug.includes('gamecube') || platformSlug === 'gc' || platformSlug === 'ngc' || platformName.includes('gamecube') || fullPath.includes('gamecube') || fullPath.includes('/ngc/');
    const isWiiPlatform = platformSlug.includes('wii') || platformName.includes('wii') || fullPath.includes('wii');
    const isDolphinCore = selectedCore.includes('dolphin') || availableCores.some(c => c.includes('dolphin'));
    const isWii = isWiiPlatform || (isDolphinCore && !isGameCube);
    const isMultiFile = !!(game?.has_multiple_files || (game?.files && game.files.length > 1) || startupFiles.length > 1);
    const hasStartupFile = isMultiFile && startupFiles.length > 0;

    const handleSelectController = useCallback((controllerId: string) => {
        setSelectedControllerType(controllerId);
        if (gameId) {
            SetGameController(gameId, controllerId).catch((err: any) => {
                console.error('Failed to set game controller:', err);
            });
        }
    }, [gameId]);

    const handleSelectStartupFile = useCallback((file: string) => {
        setSelectedStartupFile(file);
        if (gameId) {
            SetGameStartupFile(gameId, file).catch((err: any) => {
                console.error('Failed to set game startup file:', err);
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

    const closeControllerPicker = useCallback(() => {
        setIsControllerPickerOpen(false);
        setTimeout(() => setFocus('controller-selector'), 100);
    }, []);

    const closeStartupFilePicker = useCallback(() => {
        setIsStartupFilePickerOpen(false);
        setTimeout(() => setFocus('startup-file-selector'), 100);
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
                GetRomStartupFiles(game.id).then((files: string[]) => {
                    const list = files || [];
                    setStartupFiles(list);
                    GetGameStartupFile(game.id).then((saved: string) => {
                        if (saved && list.includes(saved)) {
                            setSelectedStartupFile(saved);
                        } else if (list.length > 0) {
                            setSelectedStartupFile(list[0]);
                        }
                    }).catch(() => {
                        if (list.length > 0) setSelectedStartupFile(list[0]);
                    });
                }).catch(console.error);
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
        setLoading(true);
        setStatusChecked(false);
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

            handleEscapeKey(
                e,
                isPickerOpen,
                isFirmwarePickerOpen,
                isControllerPickerOpen,
                isStartupFilePickerOpen,
                isNewSlotModalOpen,
                isDeleteSlotModalOpen,
                closePicker,
                closeFirmwarePicker,
                closeControllerPicker,
                closeStartupFilePicker,
                () => setIsNewSlotModalOpen(false),
                () => setIsDeleteSlotModalOpen(false)
            );
        };

        window.addEventListener('keydown', handleKeyDown, true);
        return () => window.removeEventListener('keydown', handleKeyDown, true);
    }, [
        isPickerOpen,
        isFirmwarePickerOpen,
        isControllerPickerOpen,
        isStartupFilePickerOpen,
        isNewSlotModalOpen,
        isDeleteSlotModalOpen,
        closePicker,
        closeFirmwarePicker,
        closeControllerPicker,
        closeStartupFilePicker,
        handleSmartSync
    ]);


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
                                                const coreCleanName = core.replace('_libretro', '').replace(/_/g, ' ');
                                                setSuccessStatus(`Switched active core to ${coreCleanName}. Saves are shared.`);
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
                    {isControllerPickerOpen && (
                        <div className="core-picker-overlay" onClick={closeControllerPicker}>
                            <div className="core-picker-modal" onClick={e => e.stopPropagation()}>
                                <div className="core-picker-header">
                                    <h3>Select Controller Type</h3>
                                </div>
                                <div className="core-picker-list">
                                    {WII_CONTROLLER_OPTIONS.map((opt, idx) => (
                                        <PickerOption
                                            key={opt.id}
                                            name={opt.name}
                                            isSelected={opt.id === selectedControllerType}
                                            isFirst={idx === 0}
                                            onSelect={() => {
                                                handleSelectController(opt.id);
                                                closeControllerPicker();
                                            }}
                                            focusKey={`controller-option-${idx}`}
                                            className="core-option"
                                        />
                                    ))}
                                </div>
                                <CancelButton onCancel={closeControllerPicker} />
                            </div>
                        </div>
                    )}
                    {isStartupFilePickerOpen && (
                        <div className="core-picker-overlay" onClick={closeStartupFilePicker}>
                            <div className="core-picker-modal" onClick={e => e.stopPropagation()}>
                                <div className="core-picker-header">
                                    <h3>Select Startup File</h3>
                                </div>
                                <div className="core-picker-list">
                                    {startupFiles.map((file, idx) => (
                                        <PickerOption
                                            key={file}
                                            name={file}
                                            isSelected={file === selectedStartupFile}
                                            isFirst={idx === 0}
                                            onSelect={() => {
                                                handleSelectStartupFile(file);
                                                closeStartupFilePicker();
                                            }}
                                            focusKey={`startup-file-option-${idx}`}
                                            className="core-option"
                                        />
                                    ))}
                                </div>
                                <CancelButton onCancel={closeStartupFilePicker} />
                            </div>
                        </div>
                    )}
                    <NewSlotModal
                        isOpen={isNewSlotModalOpen}
                        slotName={newSlotName}
                        setSlotName={setNewSlotName}
                        onConfirm={handleCreateSlotSubmit}
                        onCancel={() => setIsNewSlotModalOpen(false)}
                        inputRef={newSlotInputRef}
                    />
                    <DeleteSlotModal
                        isOpen={isDeleteSlotModalOpen}
                        activeSlot={activeSlot}
                        onConfirm={handleDeleteSlotSubmit}
                        onCancel={() => setIsDeleteSlotModalOpen(false)}
                    />
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
                                            {availableCores.length > 0 ? (
                                                <InnerCoreSelector
                                                    currentCore={selectedCore}
                                                    isDisabled={isPlaying}
                                                    hasFirmware={firmwares.length > 0}
                                                    hasController={isWii}
                                                    hasStartupFile={hasStartupFile}
                                                    hasSaves={hasSavesOrStates}
                                                    onClick={() => setIsPickerOpen(true)}
                                                    onFocusRequest={() => setFocus('core-selector')}
                                                    onFocusSaves={focusFirstAvailableSaveState}
                                                />
                                            ) : (
                                                <div className="firmware-status">No core available</div>
                                            )}
                                        </div>
                                        {isWii && (
                                            <div className="game-controller-section">
                                                <h3>Controller Type</h3>
                                                <InnerControllerSelector
                                                    currentController={selectedControllerType}
                                                    isDisabled={isPlaying}
                                                    hasCore={availableCores.length > 0}
                                                    hasFirmware={firmwares.length > 0}
                                                    hasStartupFile={hasStartupFile}
                                                    hasSaves={hasSavesOrStates}
                                                    onClick={() => setIsControllerPickerOpen(true)}
                                                    onFocusRequest={() => setFocus('controller-selector')}
                                                    onFocusSaves={focusFirstAvailableSaveState}
                                                />
                                            </div>
                                        )}
                                        {hasStartupFile && (
                                            <div className="game-startup-file-section">
                                                <h3>Startup File</h3>
                                                <InnerStartupFileSelector
                                                    currentFile={selectedStartupFile}
                                                    isDisabled={isPlaying}
                                                    hasController={isWii}
                                                    hasCore={availableCores.length > 0}
                                                    hasFirmware={firmwares.length > 0}
                                                    hasSaves={hasSavesOrStates}
                                                    onClick={() => setIsStartupFilePickerOpen(true)}
                                                    onFocusRequest={() => setFocus('startup-file-selector')}
                                                    onFocusSaves={focusFirstAvailableSaveState}
                                                />
                                            </div>
                                        )}
                                        <InnerPlayButton
                                            isDisabled={isPlaying}
                                            hasStartupFile={hasStartupFile}
                                            hasController={isWii}
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
                                    <div className="save-slots-container">
                                        <div className="save-slots-title-row">
                                            <h3>Save Slots</h3>
                                            <div className="save-slots-actions">
                                                <SlotActionButton
                                                    label="+ New Slot"
                                                    title="Create New Save Slot"
                                                    className="slot-add-btn"
                                                    focusKey="slot-btn-add"
                                                    onClick={() => {
                                                        setNewSlotName('');
                                                        setIsNewSlotModalOpen(true);
                                                    }}
                                                />
                                                {activeSlot !== "" && (
                                                    <SlotActionButton
                                                        label="Delete Slot"
                                                        title={`Delete save slot "${activeSlot}" and its server saves`}
                                                        className="slot-delete-btn"
                                                        focusKey="slot-btn-delete"
                                                        onClick={() => setIsDeleteSlotModalOpen(true)}
                                                    />
                                                )}
                                            </div>
                                        </div>
                                        <div className="save-slots-pills-row">
                                            {slots.map((s, idx) => {
                                                const isSelected = (s.slot || '') === (activeSlot || '');
                                                const displayName = s.slot === '' ? 'Legacy' : s.slot;
                                                return (
                                                    <SlotPillButton
                                                        key={s.slot || '__legacy__'}
                                                        slotName={s.slot}
                                                        displayName={displayName}
                                                        count={s.count}
                                                        isSelected={isSelected}
                                                        focusKey={`slot-pill-${idx}`}
                                                        onSelect={() => handleSelectSlot(s.slot)}
                                                    />
                                                );
                                            })}
                                        </div>
                                    </div>

                                    <h3 style={{ marginTop: '10px' }}>
                                        Server Saves {activeSlot ? `(${activeSlot === '' ? 'Legacy' : activeSlot})` : ''}
                                    </h3>
                                    <div className="file-list">
                                        {filteredServerSaves.map((save, idx) => (
                                            <FileItemRow
                                                key={`server-save-${idx}`}
                                                focusKeyPrefix={`server-save-${idx}`}
                                                item={save}
                                                onDownload={() => handleDownloadServerSave(save)}
                                                onDelete={() => handleDeleteServerSave(save.id)}
                                                status={getFileStatus(save, saves, true)}
                                                isDisabled={isPlaying || offlineMode}
                                            />
                                        ))}
                                        {(filteredServerSaves.length === 0 || offlineMode) && (
                                            <div className="no-files">
                                                <p>{offlineMode ? "Server sync unavailable offline" : `No server saves found in slot "${activeSlot || 'Legacy'}".`}</p>
                                                {!offlineMode && saves.length > 0 && (
                                                    <button
                                                        type="button"
                                                        className="btn btn-secondary"
                                                        style={{ marginTop: '8px', padding: '4px 10px', fontSize: '12px', cursor: 'pointer' }}
                                                        onClick={handleBackfillCurrentSlot}
                                                    >
                                                        Backfill slot with local save
                                                    </button>
                                                )}
                                            </div>
                                        )}
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
                                                status={getFileStatus(save, filteredServerSaves, true)}
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
                                                status={getFileStatus(state, states, false)}
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
                                                status={getFileStatus(state, serverStates, false)}
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

function InnerCoreSelector({ currentCore, isDisabled, hasFirmware, hasController, hasStartupFile, hasSaves, onClick, onFocusRequest, onFocusSaves }: {
    currentCore: string;
    isDisabled: boolean;
    hasFirmware: boolean;
    hasController?: boolean;
    hasStartupFile?: boolean;
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
                    if (hasController) setFocus('controller-selector');
                    else if (hasStartupFile) setFocus('startup-file-selector');
                    else setFocus('play-button');
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

function InnerControllerSelector({ currentController, isDisabled, hasCore, hasFirmware, hasStartupFile, hasSaves, onClick, onFocusRequest, onFocusSaves }: {
    currentController: string;
    isDisabled: boolean;
    hasCore: boolean;
    hasFirmware: boolean;
    hasStartupFile?: boolean;
    hasSaves: boolean;
    onClick: () => void;
    onFocusRequest: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'controller-selector',
        onArrowPress: (direction: string) => {
            switch (direction) {
                case 'up':
                    if (hasCore) setFocus('core-selector');
                    else if (hasFirmware) setFocus('firmware-selector');
                    return false;
                case 'down':
                    if (hasStartupFile) setFocus('startup-file-selector');
                    else setFocus('play-button');
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

    const currentOption = WII_CONTROLLER_OPTIONS.find(opt => opt.id === currentController);
    const displayName = currentOption ? currentOption.name : 'Wiimote + Nunchuk';

    return (
        <div
            id="controller-select"
            ref={ref}
            className={`controller-selector-button ${focused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
            onMouseEnter={() => {
                if (getMouseActive() && !isDisabled) {
                    onFocusRequest();
                }
            }}
            onClick={onClick}
        >
            <span className="current-controller">
                {displayName}
            </span>
            <div className="dropdown-arrow"></div>
        </div>
    );
}

function InnerStartupFileSelector({ currentFile, isDisabled, hasController, hasCore, hasFirmware, hasSaves, onClick, onFocusRequest, onFocusSaves }: {
    currentFile: string;
    isDisabled: boolean;
    hasController?: boolean;
    hasCore: boolean;
    hasFirmware: boolean;
    hasSaves: boolean;
    onClick: () => void;
    onFocusRequest: () => void;
    onFocusSaves: () => void;
}) {
    const { ref, focused } = useFocusable({
        focusKey: 'startup-file-selector',
        onArrowPress: (direction: string) => {
            switch (direction) {
                case 'up':
                    if (hasController) setFocus('controller-selector');
                    else if (hasCore) setFocus('core-selector');
                    else if (hasFirmware) setFocus('firmware-selector');
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
            id="startup-file-select"
            ref={ref}
            className={`startup-file-selector-button ${focused ? 'focused' : ''} ${isDisabled ? 'disabled' : ''}`}
            onMouseEnter={() => {
                if (getMouseActive() && !isDisabled) {
                    onFocusRequest();
                }
            }}
            onClick={onClick}
        >
            <span className="current-startup-file">
                {currentFile || 'Select File'}
            </span>
            <div className="dropdown-arrow"></div>
        </div>
    );
}

function InnerPlayButton({ isDisabled, hasStartupFile, hasController, hasCore, hasFirmware, hasSaves, onPlay, onFocusSaves }: {
    isDisabled: boolean;
    hasStartupFile?: boolean;
    hasController?: boolean;
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
                    if (hasStartupFile) setFocus('startup-file-selector');
                    else if (hasController) setFocus('controller-selector');
                    else if (hasCore) setFocus('core-selector');
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

function CancelButton({ onCancel, focusKey = 'picker-cancel' }: { onCancel: () => void; focusKey?: string }) {
    const { ref, focused } = useFocusable({
        focusKey,
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
                    setFocus(focusKey);
                }
            }}
        >
            Cancel
        </button>
    );
}

interface SlotPillButtonProps {
    slotName: string;
    displayName: string;
    count?: number;
    isSelected: boolean;
    focusKey: string;
    onSelect: () => void;
}

function SlotPillButton({
    displayName,
    count,
    isSelected,
    focusKey,
    onSelect
}: SlotPillButtonProps) {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress: onSelect,
        onArrowPress: () => true
    });

    return (
        <button
            ref={ref}
            type="button"
            className={`slot-pill-btn ${isSelected ? 'active' : ''} ${focused ? 'focused' : ''}`}
            onClick={onSelect}
            onMouseEnter={() => {
                if (getMouseActive()) {
                    setFocus(focusKey);
                }
            }}
        >
            <span className="slot-pill-name">{displayName}</span>
            {count !== undefined && count > 0 && (
                <span className="slot-pill-count">{count}</span>
            )}
        </button>
    );
}

interface SlotActionButtonProps {
    label: string;
    title: string;
    className: string;
    focusKey: string;
    onClick: () => void;
}

function SlotActionButton({
    label,
    title,
    className,
    focusKey,
    onClick
}: SlotActionButtonProps) {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress: onClick,
        onArrowPress: () => true
    });

    return (
        <button
            ref={ref}
            type="button"
            className={`btn slot-action-btn ${className} ${focused ? 'focused' : ''}`}
            onClick={onClick}
            title={title}
            onMouseEnter={() => {
                if (getMouseActive()) {
                    setFocus(focusKey);
                }
            }}
        >
            {label}
        </button>
    );
}

function NewSlotModal({
    isOpen,
    slotName,
    setSlotName,
    onConfirm,
    onCancel,
    inputRef
}: {
    isOpen: boolean;
    slotName: string;
    setSlotName: (val: string) => void;
    onConfirm: () => void;
    onCancel: () => void;
    inputRef: React.RefObject<HTMLInputElement | null>;
}) {
    if (!isOpen) return null;

    return (
        <div className="core-picker-overlay" onClick={onCancel}>
            <div className="core-picker-modal slot-dialog-modal" onClick={e => e.stopPropagation()}>
                <div className="core-picker-header">
                    <h3>New Save Slot</h3>
                </div>
                <div className="slot-modal-body">
                    <p className="slot-modal-desc">Enter a name for the new save slot:</p>
                    <input
                        ref={inputRef}
                        type="text"
                        className="slot-name-input"
                        placeholder="e.g. run-2, casual, speedrun"
                        value={slotName}
                        onChange={e => setSlotName(e.target.value)}
                        onKeyDown={e => {
                            if (e.key === 'Enter') {
                                e.preventDefault();
                                onConfirm();
                            } else if (e.key === 'Escape') {
                                e.preventDefault();
                                onCancel();
                            }
                        }}
                        maxLength={32}
                        autoFocus
                    />
                </div>
                <div className="slot-modal-actions">
                    <button
                        className="slot-confirm-btn"
                        onClick={onConfirm}
                        disabled={!slotName.trim()}
                    >
                        Create Slot
                    </button>
                    <CancelButton onCancel={onCancel} focusKey="slot-new-cancel" />
                </div>
            </div>
        </div>
    );
}

function DeleteSlotModal({
    isOpen,
    activeSlot,
    onConfirm,
    onCancel
}: {
    isOpen: boolean;
    activeSlot: string;
    onConfirm: () => void;
    onCancel: () => void;
}) {
    if (!isOpen) return null;

    return (
        <div className="core-picker-overlay" onClick={onCancel}>
            <div className="core-picker-modal slot-dialog-modal" onClick={e => e.stopPropagation()}>
                <div className="core-picker-header">
                    <h3 className="danger-text">Delete Save Slot</h3>
                </div>
                <div className="slot-modal-body">
                    <p className="slot-modal-desc">
                        Are you sure you want to delete save slot <strong>"{activeSlot}"</strong>?
                    </p>
                    <p className="slot-modal-warning">
                        This will permanently delete all saves in this slot from RomM and cannot be undone.
                    </p>
                </div>
                <div className="slot-modal-actions">
                    <button
                        className="slot-delete-confirm-btn"
                        onClick={onConfirm}
                    >
                        Delete Slot
                    </button>
                    <CancelButton onCancel={onCancel} focusKey="slot-delete-cancel" />
                </div>
            </div>
        </div>
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
