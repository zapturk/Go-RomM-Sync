import { useState, useEffect } from 'react';
import { GetConfig, SaveConfig, SelectRetroArchExecutable, DownloadAndInstallRetroArch, SelectLibraryPath, GetDefaultLibraryPath,
    Logout, ClearImageCache, ToggleOfflineMode, SyncOfflineMetadata,
    UpdateRetroArchCores, UpdateRetroArchBios, ToggleUsePlatformFolder, ToggleDisableMetadata,
    ScanOrphanedRoms, DeleteOrphanedRoms,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { FocusableButton } from './components/FocusableButton';
import { FocusableInput } from './components/FocusableInput';
import { LegendItem } from './components/LegendItem';
import { BACKGROUND_PRESETS, FONT_OPTIONS, TEXT_COLOR_PRESETS, applyTheme } from './theme';

interface SettingsProps {
    isActive?: boolean;
    onLogout?: () => void;
}

interface SettingsRowProps {
    label: string;
    desc: string;
    children: React.ReactNode;
}

function SettingsRow({ label, desc, children }: SettingsRowProps) {
    return (
        <div className="settings-row">
            <div className="settings-row-info">
                <span className="settings-row-label">{label}</span>
                <span className="settings-row-desc">{desc}</span>
            </div>
            {children}
        </div>
    );
}

function Settings({ isActive = false, onLogout }: SettingsProps) {
    const [config, setConfig] = useState<types.AppConfig | null>(null);
    const [status, setStatus] = useState("Configure your application settings");
    const [isSaving, setIsSaving] = useState(false);

    // Form states
    const [raPath, setRaPath] = useState('');
    const [libPath, setLibPath] = useState('');
    const [cheevosUser, setCheevosUser] = useState('');
    const [cheevosPass, setCheevosPass] = useState('');
    const [offlineMode, setOfflineMode] = useState(false);
    const [usePlatformFolder, setUsePlatformFolder] = useState(false);
    const [disableMetadata, setDisableMetadata] = useState(false);
    const [clientToken, setClientToken] = useState('');
    const [themeBackground, setThemeBackground] = useState('cosmic-purple');
    const [themeCustomBackground, setThemeCustomBackground] = useState('');
    const [themeFont, setThemeFont] = useState('orbitron');
    const [themeTextColor, setThemeTextColor] = useState('#ffffff');
    const [isSyncing, setIsSyncing] = useState(false);
    const [isUpdatingCores, setIsUpdatingCores] = useState(false);
    const [isUpdatingBios, setIsUpdatingBios] = useState(false);
    const [isInstallingRA, setIsInstallingRA] = useState(false);
    const [isCleaningOrphaned, setIsCleaningOrphaned] = useState(false);
    const [orphanedFiles, setOrphanedFiles] = useState<string[]>([]);
    const [showCleanupModal, setShowCleanupModal] = useState(false);

    const { ref: containerRef } = useFocusable({
        trackChildren: true,
    });

    useEffect(() => {
        GetConfig().then((cfg) => {
            const {
                retroarch_path = '',
                library_path = '',
                cheevos_username = '',
                cheevos_password = '',
                offline_mode = false,
                use_platform_folder = false,
                disable_metadata = false,
                client_token = '',
                theme_background = 'cosmic-purple',
                theme_custom_background = '',
                theme_font = 'orbitron',
                theme_text_color = '#ffffff'
            } = cfg || {};
            setConfig(cfg);
            setRaPath(retroarch_path);
            setLibPath(library_path);
            setCheevosUser(cheevos_username);
            setCheevosPass(cheevos_password);
            setOfflineMode(offline_mode);
            setUsePlatformFolder(use_platform_folder);
            setDisableMetadata(disable_metadata);
            setClientToken(client_token);
            const activeBg = theme_background || 'cosmic-purple';
            const activeCustomBg = theme_custom_background || '';
            const activeFont = theme_font || 'orbitron';
            const activeTextColor = theme_text_color || '#ffffff';
            setThemeBackground(activeBg);
            setThemeCustomBackground(activeCustomBg);
            setThemeFont(activeFont);
            setThemeTextColor(activeTextColor);
            applyTheme(activeBg, activeCustomBg, activeFont, activeTextColor);
        });
    }, []);

    useEffect(() => {
        const unsubscribeOffline = EventsOn("offline-mode-changed", (newOfflineMode: boolean) => {
            setOfflineMode(newOfflineMode);
        });

        const unsubscribeConfig = EventsOn("config-updated", () => {
            GetConfig().then((cfg) => {
                setClientToken(cfg.client_token || '');
            });
        });

        const unsubscribeBiosProgress = EventsOn("bios-download-progress", (progress: number) => {
            setStatus(`Downloading RetroArch BIOS pack (${progress}%)...`);
        });

        const unsubscribeRAProgress = EventsOn("retroarch-install-progress", (progress: number) => {
            if (progress < 100) {
                setStatus(`Downloading RetroArch (${progress}%)...`);
            } else {
                setStatus("RetroArch download complete! Installing...");
            }
        });

        return () => {
            unsubscribeOffline();
            unsubscribeConfig();
            unsubscribeBiosProgress();
            unsubscribeRAProgress();
        };
    }, []);

    // Auto-focus save button on load or when view becomes active
    useEffect(() => {
        if (isActive && config) {
            setTimeout(() => {
                setFocus('browse-ra-button');
            }, 100);
        }
    }, [isActive, !!config]);

    const handleBrowseRA = () => {
        SelectRetroArchExecutable().then((path) => {
            if (path) {
                setRaPath(path);
                setStatus("RetroArch path updated.");
            }
        });
    };

    const handleInstallRA = () => {
        if (isSaving || isInstallingRA) return;
        setIsInstallingRA(true);
        setStatus("Finding and downloading RetroArch...");
        DownloadAndInstallRetroArch()
            .then((path: string) => {
                if (path) {
                    setRaPath(path);
                    setStatus("RetroArch installed and configured successfully!");
                } else {
                    setStatus("RetroArch installation complete.");
                }
            })
            .catch((err: any) => {
                setStatus(`Error installing RetroArch: ${String(err)}`);
            })
            .finally(() => {
                setIsInstallingRA(false);
            });
    };

    const handleBrowseLib = () => {
        SelectLibraryPath().then((path) => {
            if (path) {
                setLibPath(path);
                setStatus("Library path updated.");
            }
        });
    };

    const handleSetDefaultLib = () => {
        if (isSaving) return;
        GetDefaultLibraryPath().then((path: string) => {
            if (path) {
                setLibPath(path);
                const updatedConfig = new types.AppConfig({
                    ...config,
                    library_path: path
                });
                SaveConfig(updatedConfig)
                    .then(() => {
                        setStatus("Library path set to default and saved.");
                    })
                    .catch((err: any) => {
                        setStatus(`Error saving default path: ${String(err)}`);
                    });
            }
        }).catch((err: any) => {
            setStatus(`Error getting default path: ${String(err)}`);
        });
    };

    const handleSave = () => {
        if (!config) return;

        setIsSaving(true);
        setStatus("Saving settings...");

        // We only send the updated fields, the backend SaveConfig handles merging
        const updatedConfig = new types.AppConfig({
            retroarch_path: raPath,
            library_path: libPath,
            cheevos_username: cheevosUser,
            cheevos_password: cheevosPass,
            client_token: clientToken,
            theme_background: themeBackground,
            theme_custom_background: themeCustomBackground,
            theme_font: themeFont,
            theme_text_color: themeTextColor
        });

        SaveConfig(updatedConfig)
            .then(() => {
                applyTheme(themeBackground, themeCustomBackground, themeFont, themeTextColor);
                setStatus("Settings saved successfully!");
            })
            .catch((err) => {
                setStatus(`Error: ${String(err)}`);
            })
            .finally(() => {
                setIsSaving(false);
            });
    };

    const handleSelectBackground = (bgId: string) => {
        setThemeBackground(bgId);
        applyTheme(bgId, themeCustomBackground, themeFont, themeTextColor);
        setStatus(`Background theme updated. Click Save Settings to persist.`);
    };

    const handleSelectFont = (fontId: string) => {
        setThemeFont(fontId);
        applyTheme(themeBackground, themeCustomBackground, fontId, themeTextColor);
        setStatus(`Font updated. Click Save Settings to persist.`);
    };

    const handleSelectTextColor = (color: string) => {
        setThemeTextColor(color);
        applyTheme(themeBackground, themeCustomBackground, themeFont, color);
        setStatus(`Font color updated. Click Save Settings to persist.`);
    };

    const handleCustomColorChange = (newColor: string) => {
        setThemeCustomBackground(newColor);
        setThemeBackground('custom-color');
        applyTheme('custom-color', newColor, themeFont, themeTextColor);
    };

    const handleResetAppearance = () => {
        setThemeBackground('cosmic-purple');
        setThemeCustomBackground('');
        setThemeFont('orbitron');
        setThemeTextColor('#ffffff');
        applyTheme('cosmic-purple', '', 'orbitron', '#ffffff');
        setStatus("Appearance reset to defaults. Click Save Settings to persist.");
    };

    const handleLogout = () => {
        if (isSaving) return;
        setIsSaving(true);
        setStatus("Logging out...");
        Logout()
            .then(() => {
                setStatus("Logged out successfully.");
                setCheevosUser('');
                setCheevosPass('');
                setClientToken('');
                if (onLogout) onLogout();
            })
            .catch((err: any) => {
                setStatus(`Error during logout: ${String(err)}`);
                setIsSaving(false);
            });
    };

    const handleClearCache = () => {
        if (isSaving) return;
        setIsSaving(true);
        setStatus("Clearing image cache...");
        ClearImageCache()
            .then(() => {
                setStatus("Image cache cleared successfully!");
            })
            .catch((err: any) => {
                setStatus(`Error clearing cache: ${String(err)}`);
            })
            .finally(() => {
                setIsSaving(false);
            });
    };

    const handleToggleOffline = () => {
        ToggleOfflineMode().then((newState: boolean) => {
            setOfflineMode(newState);
            setStatus(`Offline mode ${newState ? 'enabled' : 'disabled'}.`);
        });
    };

    const handleTogglePlatformFolder = () => {
        setIsSaving(true);
        setStatus("Migrating ROM library layout...");
        ToggleUsePlatformFolder()
            .then((newState: boolean) => {
                setUsePlatformFolder(newState);
                setStatus(`Library layout migration complete. Platform folder mode: ${newState ? 'Enabled' : 'Disabled'}.`);
            })
            .catch((err: any) => {
                setStatus(`Error migrating library: ${String(err)}`);
            })
            .finally(() => {
                setIsSaving(false);
            });
    };
    const handleToggleDisableMetadata = () => {
        setIsSaving(true);
        setStatus("Updating metadata configuration...");
        ToggleDisableMetadata()
            .then((newState: boolean) => {
                setDisableMetadata(newState);
                setStatus(`Metadata generation ${newState ? 'disabled' : 'enabled'}.`);
                if (newState) {
                    setOfflineMode(false);
                }
            })
            .catch((err: any) => {
                setStatus(`Error updating metadata setting: ${String(err)}`);
            })
            .finally(() => {
                setIsSaving(false);
            });
    };

    const handleSyncMetadata = () => {
        setIsSyncing(true);
        setStatus("Syncing metadata for local games...");
        SyncOfflineMetadata()
            .then(() => {
                setStatus("Metadata sync complete!");
            })
            .catch((err: any) => {
                setStatus(`Error syncing metadata: ${String(err)}`);
            })
            .finally(() => {
                setIsSyncing(false);
            });
    };

    const handleUpdateCores = () => {
        setIsUpdatingCores(true);
        setStatus("Updating RetroArch cores...");
        UpdateRetroArchCores()
            .then(() => {
                setStatus("Cores updated successfully!");
            })
            .catch((err: any) => {
                setStatus(`Error updating cores: ${String(err)}`);
            })
            .finally(() => {
                setIsUpdatingCores(false);
            });
    };

    const handleUpdateBios = () => {
        setIsUpdatingBios(true);
        setStatus("Downloading RetroArch BIOS pack...");
        UpdateRetroArchBios()
            .then(() => {
                setStatus("BIOS pack updated successfully!");
            })
            .catch((err: any) => {
                setStatus(`Error updating BIOS: ${String(err)}`);
            })
            .finally(() => {
                setIsUpdatingBios(false);
            });
    };
    const handleCleanupOrphaned = () => {
        setIsCleaningOrphaned(true);
        setStatus("Scanning library for orphaned files...");
        ScanOrphanedRoms()
            .then((files: string[]) => {
                if (!files || files.length === 0) {
                    setStatus("No orphaned files found.");
                    setIsCleaningOrphaned(false);
                } else {
                    setOrphanedFiles(files);
                    setShowCleanupModal(true);
                    setStatus(`Found ${files.length} orphaned files.`);
                    setTimeout(() => {
                        setFocus('cancel-cleanup-btn');
                    }, 100);
                }
            })
            .catch((err: any) => {
                setStatus(`Error scanning for orphans: ${String(err)}`);
                setIsCleaningOrphaned(false);
            });
    };

    const handleConfirmCleanup = () => {
        setShowCleanupModal(false);
        setStatus("Deleting orphaned files...");
        DeleteOrphanedRoms(orphanedFiles)
            .then((count: number) => {
                setStatus(`Cleanup complete! Deleted ${count} orphaned files.`);
                setOrphanedFiles([]);
            })
            .catch((err: any) => {
                setStatus(`Error deleting files: ${String(err)}`);
            })
            .finally(() => {
                setIsCleaningOrphaned(false);
            });
    };

    const handleTopArrowPress = (direction: string) => direction !== 'up';

    if (!config) return <div className="loading-screen"><h2>Loading settings...</h2></div>;

    return (
        <div id="settings-page" className="settings-page">
            <div className="settings-content" ref={containerRef}>
                <div className="settings-inner">
                    <div className="settings-header">
                        <h1>Settings</h1>
                        <div className="settings-status-box">{status}</div>
                    </div>

                    <EmulatorSection
                        raPath={raPath}
                        isSaving={isSaving}
                        isInstallingRA={isInstallingRA}
                        handleBrowseRA={handleBrowseRA}
                        handleInstallRA={handleInstallRA}
                        handleTopArrowPress={handleTopArrowPress}
                    />

                    <LibrarySection
                        libPath={libPath}
                        isSaving={isSaving}
                        usePlatformFolder={usePlatformFolder}
                        disableMetadata={disableMetadata}
                        handleBrowseLib={handleBrowseLib}
                        handleSetDefaultLib={handleSetDefaultLib}
                        handleTogglePlatformFolder={handleTogglePlatformFolder}
                        handleToggleDisableMetadata={handleToggleDisableMetadata}
                    />

                    <AppearanceSection
                        themeBackground={themeBackground}
                        themeCustomBackground={themeCustomBackground}
                        themeFont={themeFont}
                        themeTextColor={themeTextColor}
                        isSaving={isSaving}
                        onSelectBackground={handleSelectBackground}
                        onSelectFont={handleSelectFont}
                        onSelectTextColor={handleSelectTextColor}
                        onCustomColorChange={handleCustomColorChange}
                        onResetAppearance={handleResetAppearance}
                    />

                    <MaintenanceSection
                        isSaving={isSaving}
                        isUpdatingCores={isUpdatingCores}
                        isUpdatingBios={isUpdatingBios}
                        isCleaningOrphaned={isCleaningOrphaned}
                        handleClearCache={handleClearCache}
                        handleUpdateCores={handleUpdateCores}
                        handleUpdateBios={handleUpdateBios}
                        handleCleanupOrphaned={handleCleanupOrphaned}
                    />

                    <OfflineSection
                        isSaving={isSaving}
                        isSyncing={isSyncing}
                        offlineMode={offlineMode}
                        disableMetadata={disableMetadata}
                        handleToggleOffline={handleToggleOffline}
                        handleSyncMetadata={handleSyncMetadata}
                    />

                    <RetroAchievementsSection
                        cheevosUser={cheevosUser}
                        setCheevosUser={setCheevosUser}
                        cheevosPass={cheevosPass}
                        setCheevosPass={setCheevosPass}
                    />

                    <RomMConnectionSection
                        clientToken={clientToken}
                        setClientToken={setClientToken}
                    />

                    <div className="settings-actions">
                        <FocusableButton
                            focusKey="save-button"
                            className="btn btn-primary"
                            style={{
                                flex: 1,
                                height: '50px',
                                fontSize: '1.2rem',
                                margin: 0
                            }}
                            onClick={handleSave}
                            onEnterPress={handleSave}
                            disabled={isSaving}
                            onMouseEnter={() => getMouseActive() && setFocus('save-button')}
                        >
                            {isSaving ? "Saving..." : "Save Settings"}
                        </FocusableButton>
                        <FocusableButton
                            focusKey="logout-button"
                            className="btn btn-danger"
                            style={{
                                flex: 1,
                                height: '50px',
                                fontSize: '1.2rem',
                                margin: 0
                            }}
                            onClick={handleLogout}
                            onEnterPress={handleLogout}
                            disabled={isSaving}
                            onMouseEnter={() => getMouseActive() && !isSaving && setFocus('logout-button')}
                        >
                            Logout
                        </FocusableButton>
                    </div>
                </div>
            </div>

            <div className="input-legend">
                <div className="footer-left">
                    <span>{status}</span>
                </div>
                <div className="footer-right">
                    <LegendItem buttonAction="east" keyLabel="ESC" label="Back" />
                    <LegendItem buttonAction="south" keyLabel="ENTER" label="OK" />
                </div>
            </div>
            {showCleanupModal && (
                <div className="core-picker-overlay">
                    <div className="cleanup-modal">
                        <h3>Confirm Deletion</h3>
                        <p>The following {orphanedFiles.length} files/folders are not tracked in your local metadata and will be deleted:</p>
                        <div className="orphaned-list">
                            {orphanedFiles.map(file => (
                                <div key={file} className="orphaned-file-item">{file}</div>
                            ))}
                        </div>
                        <div className="modal-actions">
                            <FocusableButton
                                focusKey="confirm-cleanup-btn"
                                className="btn btn-danger"
                                onClick={handleConfirmCleanup}
                                onEnterPress={handleConfirmCleanup}
                                onMouseEnter={() => getMouseActive() && setFocus('confirm-cleanup-btn')}
                            >
                                Yes, Delete
                            </FocusableButton>
                            <FocusableButton
                                focusKey="cancel-cleanup-btn"
                                className="btn"
                                onClick={() => {
                                    setShowCleanupModal(false);
                                    setIsCleaningOrphaned(false);
                                    setStatus("Configure your application settings");
                                }}
                                onEnterPress={() => {
                                    setShowCleanupModal(false);
                                    setIsCleaningOrphaned(false);
                                    setStatus("Configure your application settings");
                                }}
                                onMouseEnter={() => getMouseActive() && setFocus('cancel-cleanup-btn')}
                            >
                                Cancel
                            </FocusableButton>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

interface EmulatorSectionProps {
    raPath: string;
    isSaving: boolean;
    isInstallingRA: boolean;
    handleBrowseRA: () => void;
    handleInstallRA: () => void;
    handleTopArrowPress: (direction: string) => boolean;
}

function EmulatorSection({ raPath, isSaving, isInstallingRA, handleBrowseRA, handleInstallRA, handleTopArrowPress }: EmulatorSectionProps) {
    return (
        <div className="settings-card">
            <div className="settings-section-title">Emulator Configuration</div>
            <div className="input-group">
                <label>RetroArch Executable</label>
                <div>
                    <FocusableInput
                        className="input"
                        value={raPath}
                        readOnly
                        placeholder="Not configured"
                        focusKey="ra-path-input"
                        onArrowPress={handleTopArrowPress}
                    />
                    <FocusableButton
                        focusKey="browse-ra-button"
                        className={`btn ${isSaving || isInstallingRA ? 'disabled' : ''}`}
                        onClick={handleBrowseRA}
                        onEnterPress={handleBrowseRA}
                        onArrowPress={handleTopArrowPress}
                        disabled={isSaving || isInstallingRA}
                        onMouseEnter={() => getMouseActive() && !isSaving && !isInstallingRA && setFocus('browse-ra-button')}
                    >
                        Browse
                    </FocusableButton>
                    <FocusableButton
                        focusKey="install-ra-button"
                        className={`btn ${isSaving || isInstallingRA ? 'disabled' : ''}`}
                        onClick={handleInstallRA}
                        onEnterPress={handleInstallRA}
                        onArrowPress={handleTopArrowPress}
                        disabled={isSaving || isInstallingRA}
                        onMouseEnter={() => getMouseActive() && !isSaving && !isInstallingRA && setFocus('install-ra-button')}
                    >
                        {isInstallingRA ? "Installing..." : "Install RetroArch"}
                    </FocusableButton>
                </div>
            </div>
        </div>
    );
}

interface LibrarySectionProps {
    libPath: string;
    isSaving: boolean;
    usePlatformFolder: boolean;
    disableMetadata: boolean;
    handleBrowseLib: () => void;
    handleSetDefaultLib: () => void;
    handleTogglePlatformFolder: () => void;
    handleToggleDisableMetadata: () => void;
}

function LibrarySection({
    libPath,
    isSaving,
    usePlatformFolder,
    disableMetadata,
    handleBrowseLib,
    handleSetDefaultLib,
    handleTogglePlatformFolder,
    handleToggleDisableMetadata
}: LibrarySectionProps) {
    const toggleStyle = {
        minWidth: '120px',
        backgroundColor: usePlatformFolder ? '#4CAF50' : 'rgba(255,255,255,0.1)',
    };

    const disableMetaStyle = {
        minWidth: '120px',
        backgroundColor: disableMetadata ? '#f44336' : 'rgba(255,255,255,0.1)',
    };

    return (
        <div className="settings-card">
            <div className="settings-section-title">Library Configuration</div>
            <div className="input-group">
                <label>Local ROM Library Path</label>
                <div>
                    <FocusableInput
                        className="input"
                        value={libPath}
                        readOnly
                        placeholder="Not configured"
                        focusKey="lib-path-input"
                    />
                    <FocusableButton
                        focusKey="browse-lib-button"
                        className={`btn ${isSaving ? 'disabled' : ''}`}
                        onClick={handleBrowseLib}
                        onEnterPress={handleBrowseLib}
                        disabled={isSaving}
                        onMouseEnter={() => getMouseActive() && !isSaving && setFocus('browse-lib-button')}
                    >
                        Browse
                    </FocusableButton>
                    <FocusableButton
                        focusKey="default-lib-button"
                        className={`btn ${isSaving ? 'disabled' : ''}`}
                        onClick={handleSetDefaultLib}
                        onEnterPress={handleSetDefaultLib}
                        disabled={isSaving}
                        onMouseEnter={() => getMouseActive() && !isSaving && setFocus('default-lib-button')}
                    >
                        Set Default
                    </FocusableButton>
                </div>
            </div>
            <SettingsRow label="Use Platform Folder" desc="Store ROMs directly in the platform folder, omitting the ID subfolder">
                <FocusableButton
                    focusKey="platform-folder-toggle-button"
                    className={`btn ${isSaving ? 'disabled' : ''}`}
                    style={toggleStyle}
                    onClick={handleTogglePlatformFolder}
                    onEnterPress={handleTogglePlatformFolder}
                    disabled={isSaving}
                    onMouseEnter={() => getMouseActive() && !isSaving && setFocus('platform-folder-toggle-button')}
                >
                    {usePlatformFolder ? "Enabled" : "Disabled"}
                </FocusableButton>
            </SettingsRow>
            <SettingsRow label="Disable Metadata" desc="Disable generation of metadata.json files (this disables Offline Mode)">
                <FocusableButton
                    focusKey="disable-metadata-toggle-button"
                    className={`btn ${isSaving ? 'disabled' : ''}`}
                    style={disableMetaStyle}
                    onClick={handleToggleDisableMetadata}
                    onEnterPress={handleToggleDisableMetadata}
                    disabled={isSaving}
                    onMouseEnter={() => getMouseActive() && !isSaving && setFocus('disable-metadata-toggle-button')}
                >
                    {disableMetadata ? "Disabled" : "Enabled"}
                </FocusableButton>
            </SettingsRow>
        </div>
    );
}

const handleHover = (focusKey: string, isSaving: boolean, isUpdating?: boolean) => {
    if (!getMouseActive()) return;
    if (isSaving || isUpdating) return;
    setFocus(focusKey);
};

const getBtnClassName = (isSaving: boolean, isUpdating?: boolean) => {
    const disabled = isSaving || isUpdating;
    return `btn ${disabled ? 'disabled' : ''}`;
};

const handleOfflineArrowPress = (direction: string) => {
    if (direction === 'up') {
        setFocus('update-cores-button');
        return false;
    }
    return true;
};

interface MaintenanceSectionProps {
    isSaving: boolean;
    isUpdatingCores: boolean;
    isUpdatingBios: boolean;
    isCleaningOrphaned: boolean;
    handleClearCache: () => void;
    handleUpdateCores: () => void;
    handleUpdateBios: () => void;
    handleCleanupOrphaned: () => void;
}

function MaintenanceSection({
    isSaving,
    isUpdatingCores,
    isUpdatingBios,
    isCleaningOrphaned,
    handleClearCache,
    handleUpdateCores,
    handleUpdateBios,
    handleCleanupOrphaned
}: MaintenanceSectionProps) {
    return (
        <div className="settings-card">
            <div className="settings-section-title">Maintenance</div>
            <SettingsRow label="Local Image Cache" desc="Refresh game covers and screenshots">
                <FocusableButton
                    focusKey="clear-cache-button"
                    className={getBtnClassName(isSaving)}
                    onClick={handleClearCache}
                    onEnterPress={handleClearCache}
                    disabled={isSaving}
                    onMouseEnter={() => handleHover('clear-cache-button', isSaving)}
                >
                    Clear Cache
                </FocusableButton>
            </SettingsRow>
            <SettingsRow label="Update Cores" desc="Re-download all downloaded cores for the latest updates">
                <FocusableButton
                    focusKey="update-cores-button"
                    className={getBtnClassName(isSaving, isUpdatingCores)}
                    onClick={handleUpdateCores}
                    onEnterPress={handleUpdateCores}
                    disabled={isSaving || isUpdatingCores}
                    onMouseEnter={() => handleHover('update-cores-button', isSaving, isUpdatingCores)}
                >
                    {isUpdatingCores ? "Updating..." : "Update Cores"}
                </FocusableButton>
            </SettingsRow>
            <SettingsRow label="Download BIOS" desc="Download latest RetroArch BIOS pack from Abdess/retrobios">
                <FocusableButton
                    focusKey="update-bios-button"
                    className={getBtnClassName(isSaving, isUpdatingBios)}
                    onClick={handleUpdateBios}
                    onEnterPress={handleUpdateBios}
                    disabled={isSaving || isUpdatingBios}
                    onMouseEnter={() => handleHover('update-bios-button', isSaving, isUpdatingBios)}
                >
                    {isUpdatingBios ? "Downloading..." : "Download BIOS"}
                </FocusableButton>
            </SettingsRow>
            <SettingsRow label="Clean Up Orphaned ROMs" desc="Delete downloaded ROMs/saves not present in metadata database">
                <FocusableButton
                    focusKey="cleanup-orphans-button"
                    className={getBtnClassName(isSaving, isCleaningOrphaned)}
                    onClick={handleCleanupOrphaned}
                    onEnterPress={handleCleanupOrphaned}
                    disabled={isSaving || isCleaningOrphaned}
                    onMouseEnter={() => handleHover('cleanup-orphans-button', isSaving, isCleaningOrphaned)}
                >
                    {isCleaningOrphaned ? "Cleaning..." : "Clean Up"}
                </FocusableButton>
            </SettingsRow>
        </div>
    );
}

interface OfflineSectionProps {
    isSaving: boolean;
    isSyncing: boolean;
    offlineMode: boolean;
    disableMetadata: boolean;
    handleToggleOffline: () => void;
    handleSyncMetadata: () => void;
}

function OfflineSection({
    isSaving,
    isSyncing,
    offlineMode,
    disableMetadata,
    handleToggleOffline,
    handleSyncMetadata
}: OfflineSectionProps) {
    const toggleStyle = {
        minWidth: '120px',
        backgroundColor: (offlineMode && !disableMetadata) ? '#4CAF50' : 'rgba(255,255,255,0.1)',
    };

    return (
        <div className="settings-card">
            <div className="settings-section-title">Offline Support</div>
            <SettingsRow 
                label="Offline Mode" 
                desc={disableMetadata ? "Offline Mode requires metadata.json files (currently disabled)" : "Enable browsing without server connection"}
            >
                <FocusableButton
                    focusKey="offline-toggle-button"
                    className={getBtnClassName(isSaving || disableMetadata)}
                    style={toggleStyle}
                    onClick={handleToggleOffline}
                    onEnterPress={handleToggleOffline}
                    onArrowPress={handleOfflineArrowPress}
                    disabled={isSaving || disableMetadata}
                    onMouseEnter={() => handleHover('offline-toggle-button', isSaving || disableMetadata)}
                >
                    {disableMetadata ? "Unavailable" : (offlineMode ? "Enabled" : "Disabled")}
                </FocusableButton>
            </SettingsRow>
            <SettingsRow label="Sync Metadata" desc={disableMetadata ? "Sync is unavailable when metadata is disabled" : "Prepare game data for offline use"}>
                <FocusableButton
                    focusKey="sync-metadata-button"
                    className={getBtnClassName(isSaving || disableMetadata, isSyncing)}
                    onClick={handleSyncMetadata}
                    onEnterPress={handleSyncMetadata}
                    disabled={isSaving || isSyncing || disableMetadata}
                    onMouseEnter={() => handleHover('sync-metadata-button', isSaving || disableMetadata, isSyncing)}
                >
                    {isSyncing ? "Syncing..." : "Sync Metadata"}
                </FocusableButton>
            </SettingsRow>
        </div>
    );
}

interface RetroAchievementsSectionProps {
    cheevosUser: string;
    setCheevosUser: (val: string) => void;
    cheevosPass: string;
    setCheevosPass: (val: string) => void;
}

function RetroAchievementsSection({
    cheevosUser,
    setCheevosUser,
    cheevosPass,
    setCheevosPass
}: RetroAchievementsSectionProps) {
    return (
        <div className="settings-card">
            <div className="settings-section-title">RetroAchievements</div>
            <div className="input-group">
                <label htmlFor="cheevosUser">Username</label>
                <FocusableInput
                    id="cheevosUser"
                    focusKey="cheevos-user-input"
                    className="input"
                    value={cheevosUser}
                    onChange={(e) => setCheevosUser(e.target.value)}
                    autoComplete="off"
                />
            </div>
            <div className="input-group">
                <label htmlFor="cheevosPass">Password</label>
                <FocusableInput
                    id="cheevosPass"
                    focusKey="cheevos-pass-input"
                    className="input"
                    type="password"
                    value={cheevosPass}
                    onChange={(e) => setCheevosPass(e.target.value)}
                    autoComplete="off"
                />
            </div>
        </div>
    );
}

interface RomMConnectionSectionProps {
    clientToken: string;
    setClientToken: (val: string) => void;
}

function RomMConnectionSection({ clientToken, setClientToken }: RomMConnectionSectionProps) {
    return (
        <div className="settings-card">
            <div className="settings-section-title">RomM Connection</div>
            <div className="input-group">
                <label htmlFor="clientToken">Client Token</label>
                <FocusableInput
                    id="clientToken"
                    focusKey="client-token-input"
                    className="input"
                    value={clientToken}
                    onChange={(e) => setClientToken(e.target.value)}
                    autoComplete="off"
                    placeholder="rmm_..."
                />
                <div className="input-help-text" style={{ fontSize: '0.8rem', opacity: 0.7, marginTop: '0.5rem' }}>
                    A persistent token for stable connection. The app can auto-generate this if you login normally, or you can paste one from RomM Settings.
                </div>
            </div>
        </div>
    );
}

interface AppearanceSectionProps {
    themeBackground: string;
    themeCustomBackground: string;
    themeFont: string;
    themeTextColor: string;
    isSaving: boolean;
    onSelectBackground: (bgId: string) => void;
    onSelectFont: (fontId: string) => void;
    onSelectTextColor: (color: string) => void;
    onCustomColorChange: (color: string) => void;
    onResetAppearance: () => void;
}

function AppearanceSection({
    themeBackground,
    themeCustomBackground,
    themeFont,
    themeTextColor,
    isSaving,
    onSelectBackground,
    onSelectFont,
    onSelectTextColor,
    onCustomColorChange,
    onResetAppearance
}: AppearanceSectionProps) {
    return (
        <div className="settings-card">
            <div className="settings-section-title">Appearance & Themes</div>

            <div className="settings-row" style={{ alignItems: 'flex-start' }}>
                <div className="settings-row-info">
                    <span className="settings-row-label">Background Theme</span>
                    <span className="settings-row-desc">
                        Select a curated color theme, or customize with your own solid color or gradient
                    </span>
                </div>
            </div>

            <div className="theme-grid">
                {BACKGROUND_PRESETS.map((preset) => {
                    const isSelected = themeBackground === preset.id;
                    return (
                        <FocusableButton
                            key={preset.id}
                            focusKey={`theme-preset-${preset.id}`}
                            className={`theme-preset-btn ${isSelected ? 'active' : ''}`}
                            onClick={() => onSelectBackground(preset.id)}
                            onEnterPress={() => onSelectBackground(preset.id)}
                            onMouseEnter={() => getMouseActive() && setFocus(`theme-preset-${preset.id}`)}
                            title={preset.description}
                        >
                            <span className="theme-swatch" style={{ background: preset.preview }} />
                            <span className="theme-preset-name">{preset.name}</span>
                        </FocusableButton>
                    );
                })}
                <FocusableButton
                    focusKey="theme-preset-custom-color"
                    className={`theme-preset-btn ${themeBackground === 'custom-color' ? 'active' : ''}`}
                    onClick={() => onSelectBackground('custom-color')}
                    onEnterPress={() => onSelectBackground('custom-color')}
                    onMouseEnter={() => getMouseActive() && setFocus('theme-preset-custom-color')}
                    title="Choose a custom solid color or gradient"
                >
                    <span
                        className="theme-swatch"
                        style={{
                            background: themeBackground === 'custom-color' && themeCustomBackground ? themeCustomBackground : 'linear-gradient(45deg, #f06, #4a90e2)'
                        }}
                    />
                    <span className="theme-preset-name">Custom Color</span>
                </FocusableButton>
            </div>

            {themeBackground === 'custom-color' && (
                <div className="custom-theme-controls">
                    <span className="settings-row-label">Custom Background Color</span>
                    <div className="color-picker-row">
                        <input
                            type="color"
                            className="native-color-input"
                            value={themeCustomBackground.startsWith('#') && themeCustomBackground.length === 7 ? themeCustomBackground : '#121212'}
                            onChange={(e) => onCustomColorChange(e.target.value)}
                            title="Choose color"
                        />
                        <FocusableInput
                            focusKey="custom-color-input"
                            className="input"
                            value={themeCustomBackground}
                            onChange={(e) => onCustomColorChange(e.target.value)}
                            placeholder="#121212 or any valid CSS color"
                            style={{ flex: 1 }}
                        />
                    </div>
                </div>
            )}

            <div className="settings-row" style={{ alignItems: 'flex-start', marginTop: '16px' }}>
                <div className="settings-row-info">
                    <span className="settings-row-label">Application Font</span>
                    <span className="settings-row-desc">
                        Choose your preferred typography style across the entire application
                    </span>
                </div>
            </div>

            <div className="font-grid">
                {FONT_OPTIONS.map((font) => {
                    const isSelected = themeFont === font.id;
                    return (
                        <FocusableButton
                            key={font.id}
                            focusKey={`font-option-${font.id}`}
                            className={`font-option-btn ${isSelected ? 'active' : ''}`}
                            onClick={() => onSelectFont(font.id)}
                            onEnterPress={() => onSelectFont(font.id)}
                            onMouseEnter={() => getMouseActive() && setFocus(`font-option-${font.id}`)}
                            title={font.description}
                        >
                            <div className="font-option-title" style={{ fontFamily: font.family }}>
                                <span>{font.name}</span>
                                {isSelected && <span className="font-active-badge" />}
                            </div>
                            <div className="font-option-preview" style={{ fontFamily: font.family }}>
                                {font.previewText}
                            </div>
                        </FocusableButton>
                    );
                })}
            </div>

            {/* Font Color selection */}
            <div className="settings-row" style={{ alignItems: 'flex-start', marginTop: '16px' }}>
                <div className="settings-row-info">
                    <span className="settings-row-label">Font Color</span>
                    <span className="settings-row-desc">
                        Customize the text color across the entire application interface
                    </span>
                </div>
            </div>

            <div className="color-swatch-grid">
                {TEXT_COLOR_PRESETS.map((colorItem) => {
                    const isSelected = themeTextColor.toLowerCase() === colorItem.color.toLowerCase();
                    return (
                        <FocusableButton
                            key={colorItem.id}
                            focusKey={`color-preset-${colorItem.id}`}
                            className={`color-swatch-btn ${isSelected ? 'active' : ''}`}
                            onClick={() => onSelectTextColor(colorItem.color)}
                            onEnterPress={() => onSelectTextColor(colorItem.color)}
                            onMouseEnter={() => getMouseActive() && setFocus(`color-preset-${colorItem.id}`)}
                        >
                            <span className="font-color-dot" style={{ backgroundColor: colorItem.color }} />
                            <span>{colorItem.name}</span>
                        </FocusableButton>
                    );
                })}
            </div>

            <div className="custom-theme-controls" style={{ marginTop: '10px' }}>
                <span className="settings-row-label">Custom Font Color</span>
                <div className="color-picker-row">
                    <input
                        type="color"
                        className="native-color-input"
                        value={themeTextColor.startsWith('#') && themeTextColor.length === 7 ? themeTextColor : '#ffffff'}
                        onChange={(e) => onSelectTextColor(e.target.value)}
                        title="Pick custom font color"
                    />
                    <FocusableInput
                        focusKey="custom-text-color-input"
                        className="input"
                        value={themeTextColor}
                        onChange={(e) => onSelectTextColor(e.target.value)}
                        placeholder="#ffffff or any CSS color"
                        style={{ flex: 1 }}
                    />
                </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '14px' }}>
                <FocusableButton
                    focusKey="reset-appearance-btn"
                    className="btn"
                    style={{ fontSize: '0.85rem', padding: '0 16px', height: '38px' }}
                    onClick={onResetAppearance}
                    onEnterPress={onResetAppearance}
                    disabled={isSaving}
                    onMouseEnter={() => getMouseActive() && setFocus('reset-appearance-btn')}
                >
                    Reset Appearance
                </FocusableButton>
            </div>
        </div>
    );
}

export default Settings;
