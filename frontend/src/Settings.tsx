import { useState, useEffect } from 'react';
// @ts-ignore
import { GetConfig, SaveConfig, SelectRetroArchExecutable, SelectLibraryPath, GetDefaultLibraryPath, Logout, ClearImageCache, ToggleOfflineMode, SyncOfflineMetadata } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { FocusableButton } from './components/FocusableButton';
import { FocusableInput } from './components/FocusableInput';

interface SettingsProps {
    isActive?: boolean;
    onLogout?: () => void;
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
    const [isSyncing, setIsSyncing] = useState(false);

    useEffect(() => {
        GetConfig().then((cfg) => {
            setConfig(cfg);
            setRaPath(cfg.retroarch_path || '');
            setLibPath(cfg.library_path || '');
            setCheevosUser(cfg.cheevos_username || '');
            setCheevosPass(cfg.cheevos_password || '');
            // @ts-ignore
            setOfflineMode(cfg.offline_mode || false);
        });
    }, []);

    useEffect(() => {
        const unsubscribe = EventsOn("offline-mode-changed", (newOfflineMode: boolean) => {
            setOfflineMode(newOfflineMode);
        });
        return () => unsubscribe();
    }, []);

    const handleBrowseRA = () => {
        SelectRetroArchExecutable().then((path) => {
            if (path) {
                setRaPath(path);
                setStatus("RetroArch path updated.");
            }
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
                        setStatus("Error saving default path: " + err);
                    });
            }
        }).catch((err: any) => {
            setStatus("Error getting default path: " + err);
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
            cheevos_password: cheevosPass
        });

        SaveConfig(updatedConfig)
            .then((res) => {
                setStatus("Settings saved successfully!");
            })
            .catch((err) => {
                setStatus("Error: " + err);
            })
            .finally(() => {
                setIsSaving(false);
            });
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
                if (onLogout) onLogout();
            })
            .catch((err: any) => {
                setStatus("Error during logout: " + err);
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
                setStatus("Error clearing cache: " + err);
            })
            .finally(() => {
                setIsSaving(false);
            });
    };

    const handleToggleOffline = () => {
        // @ts-ignore
        ToggleOfflineMode().then((newState: boolean) => {
            setOfflineMode(newState);
            setStatus(`Offline mode ${newState ? 'enabled' : 'disabled'}.`);
        });
    };

    const handleSyncMetadata = () => {
        setIsSyncing(true);
        setStatus("Syncing metadata for local games...");
        // @ts-ignore
        SyncOfflineMetadata()
            .then(() => {
                setStatus("Metadata sync complete!");
            })
            .catch((err: any) => {
                setStatus("Error syncing metadata: " + err);
            })
            .finally(() => {
                setIsSyncing(false);
            });
    };

    const handleInputKeyDown = (e: React.KeyboardEvent) => {
        if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) {
            e.stopPropagation();
        }
    };

    if (!config) return <div className="loading-screen"><h2>Loading settings...</h2></div>;

    return (
        <SettingsForm
            config={config}
            isActive={isActive}
            onLogout={onLogout}
            raPath={raPath}
            setRaPath={setRaPath}
            libPath={libPath}
            setLibPath={setLibPath}
            cheevosUser={cheevosUser}
            setCheevosUser={setCheevosUser}
            cheevosPass={cheevosPass}
            setCheevosPass={setCheevosPass}
            status={status}
            setStatus={setStatus}
            isSaving={isSaving}
            handleBrowseRA={handleBrowseRA}
            handleBrowseLib={handleBrowseLib}
            handleSetDefaultLib={handleSetDefaultLib}
            handleSave={handleSave}
            handleLogout={handleLogout}
            handleClearCache={handleClearCache}
            offlineMode={offlineMode}
            handleToggleOffline={handleToggleOffline}
            handleSyncMetadata={handleSyncMetadata}
            isSyncing={isSyncing}
        />
    );
}

interface SettingsFormProps {
    config: types.AppConfig;
    isActive: boolean;
    onLogout?: () => void;
    raPath: string;
    setRaPath: (v: string) => void;
    libPath: string;
    setLibPath: (v: string) => void;
    cheevosUser: string;
    setCheevosUser: (v: string) => void;
    cheevosPass: string;
    setCheevosPass: (v: string) => void;
    status: string;
    setStatus: (v: string) => void;
    isSaving: boolean;
    handleBrowseRA: () => void;
    handleBrowseLib: () => void;
    handleSetDefaultLib: () => void;
    handleSave: () => void;
    handleLogout: () => void;
    handleClearCache: () => void;
    offlineMode: boolean;
    handleToggleOffline: () => void;
    handleSyncMetadata: () => void;
    isSyncing: boolean;
}

function SettingsForm({
    isActive,
    onLogout,
    raPath,
    setRaPath,
    libPath,
    setLibPath,
    cheevosUser,
    setCheevosUser,
    cheevosPass,
    setCheevosPass,
    status,
    setStatus,
    isSaving,
    handleBrowseRA,
    handleBrowseLib,
    handleSetDefaultLib,
    handleSave,
    handleLogout,
    handleClearCache,
    offlineMode,
    handleToggleOffline,
    handleSyncMetadata,
    isSyncing
}: SettingsFormProps) {
    const { ref: containerRef } = useFocusable({
        trackChildren: true,
    });

    // Auto-focus save button on load or when view becomes active
    useEffect(() => {
        if (isActive) {
            setTimeout(() => {
                setFocus('save-button');
            }, 100);
        }
    }, [isActive]);

    // Focus management handled by components

    return (
        <div id="settings-page" className="settings-page">
            <div className="settings-content" ref={containerRef}>
                <div className="settings-inner">
                    <div className="settings-header">
                        <h1>Settings</h1>
                        <div className="settings-status-box">{status}</div>
                    </div>

                    {/* Emulator Section */}
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
                                />
                                <FocusableButton
                                    focusKey="browse-ra-button"
                                    className={`btn ${isSaving ? 'disabled' : ''}`}
                                    onClick={handleBrowseRA}
                                    onEnterPress={handleBrowseRA}
                                    disabled={isSaving}
                                    onMouseEnter={() => getMouseActive() && !isSaving && setFocus('browse-ra-button')}
                                >
                                    Browse
                                </FocusableButton>
                            </div>
                        </div>
                    </div>

                    {/* Library Section */}
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
                    </div>

                    {/* Maintenance Section */}
                    <div className="settings-card">
                        <div className="settings-section-title">Maintenance</div>
                        <div className="settings-row">
                            <div className="settings-row-info">
                                <span className="settings-row-label">Local Image Cache</span>
                                <span className="settings-row-desc">Refresh game covers and screenshots</span>
                            </div>
                            <FocusableButton
                                focusKey="clear-cache-button"
                                className={`btn ${isSaving ? 'disabled' : ''}`}
                                onClick={handleClearCache}
                                onEnterPress={handleClearCache}
                                disabled={isSaving}
                                onMouseEnter={() => getMouseActive() && !isSaving && setFocus('clear-cache-button')}
                            >
                                Clear Cache
                            </FocusableButton>
                        </div>
                    </div>

                    {/* Offline Support Section */}
                    <div className="settings-card">
                        <div className="settings-section-title">Offline Support</div>
                        <div className="settings-row">
                            <div className="settings-row-info">
                                <span className="settings-row-label">Offline Mode</span>
                                <span className="settings-row-desc">Enable browsing without server connection</span>
                            </div>
                            <FocusableButton
                                focusKey="offline-toggle-button"
                                className={`btn ${isSaving ? 'disabled' : ''}`}
                                style={{
                                    minWidth: '120px',
                                    backgroundColor: offlineMode ? '#4CAF50' : 'rgba(255,255,255,0.1)',
                                }}
                                onClick={handleToggleOffline}
                                onEnterPress={handleToggleOffline}
                                disabled={isSaving}
                                onMouseEnter={() => getMouseActive() && !isSaving && setFocus('offline-toggle-button')}
                            >
                                {offlineMode ? "Enabled" : "Disabled"}
                            </FocusableButton>
                        </div>
                        <div className="settings-row">
                            <div className="settings-row-info">
                                <span className="settings-row-label">Sync Metadata</span>
                                <span className="settings-row-desc">Prepare game data for offline use</span>
                            </div>
                            <FocusableButton
                                focusKey="sync-metadata-button"
                                className={`btn ${isSaving || isSyncing ? 'disabled' : ''}`}
                                onClick={handleSyncMetadata}
                                onEnterPress={handleSyncMetadata}
                                disabled={isSaving || isSyncing}
                                onMouseEnter={() => getMouseActive() && !isSaving && !isSyncing && setFocus('sync-metadata-button')}
                            >
                                {isSyncing ? "Syncing..." : "Sync Now"}
                            </FocusableButton>
                        </div>
                    </div>

                    {/* RetroAchievements Section */}
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

export default Settings;
