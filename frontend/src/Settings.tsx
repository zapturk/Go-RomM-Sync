import { useState, useEffect } from 'react';
import { GetConfig, SaveConfig, SelectRetroArchExecutable, SelectLibraryPath, GetDefaultLibraryPath,
    Logout, ClearImageCache, ToggleOfflineMode, SyncOfflineMetadata,
    UpdateRetroArchCores, UpdateRetroArchBios,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime";
import { types } from "../wailsjs/go/models";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';
import { FocusableButton } from './components/FocusableButton';
import { FocusableInput } from './components/FocusableInput';
import { LegendItem } from './components/LegendItem';

interface SettingsProps {
    isActive?: boolean;
    onLogout?: () => void;
}

function Settings({ isActive = false, onLogout }: SettingsProps) {
    const [config, setConfig] = useState<types.AppConfig | null>(null);
    const [status, setStatus] = useState("Configure your application settings");
    const [isSaving, setIsSaving] = useState(false);

    // Form states
    const [form, setForm] = useState({
        raPath: '',
        libPath: '',
        cheevosUser: '',
        cheevosPass: '',
        offlineMode: false,
        clientToken: '',
    });
    
    const [isSyncing, setIsSyncing] = useState(false);
    const [isUpdatingCores, setIsUpdatingCores] = useState(false);
    const [isUpdatingBios, setIsUpdatingBios] = useState(false);

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
                client_token = ''
            } = cfg || {};
            setConfig(cfg);
            setForm({
                raPath: retroarch_path,
                libPath: library_path,
                cheevosUser: cheevos_username,
                cheevosPass: cheevos_password,
                offlineMode: offline_mode,
                clientToken: client_token,
            });
        });
    }, []);

    useEffect(() => {
        const unsubscribeOffline = EventsOn("offline-mode-changed", (newOfflineMode: boolean) => {
            setForm(prev => ({ ...prev, offlineMode: newOfflineMode }));
            setStatus(`Offline mode ${newOfflineMode ? 'enabled' : 'disabled'}.`);
        });

        const unsubscribeConfig = EventsOn("config-updated", () => {
            GetConfig().then((cfg) => {
                setForm(prev => ({ ...prev, clientToken: cfg.client_token || '' }));
            });
        });

        return () => {
            unsubscribeOffline();
            unsubscribeConfig();
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
                setForm(prev => ({ ...prev, raPath: path }));
                setStatus("RetroArch path updated.");
            }
        });
    };

    const handleBrowseLib = () => {
        SelectLibraryPath().then((path) => {
            if (path) {
                setForm(prev => ({ ...prev, libPath: path }));
                setStatus("Library path updated.");
            }
        });
    };

    const handleSetDefaultLib = () => {
        if (isSaving) return;
        GetDefaultLibraryPath().then((path: string) => {
            if (path) {
                setForm(prev => ({ ...prev, libPath: path }));
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

        const updatedConfig = new types.AppConfig({
            retroarch_path: form.raPath,
            library_path: form.libPath,
            cheevos_username: form.cheevosUser,
            cheevos_password: form.cheevosPass,
            client_token: form.clientToken
        });

        SaveConfig(updatedConfig)
            .then(() => {
                setStatus("Settings saved successfully!");
            })
            .catch((err) => {
                setStatus(`Error: ${String(err)}`);
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
                setForm(prev => ({ ...prev, cheevosUser: '', cheevosPass: '', clientToken: '' }));
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
            setForm(prev => ({ ...prev, offlineMode: newState }));
            setStatus(`Offline mode ${newState ? 'enabled' : 'disabled'}.`);
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

                    {/* Emulator Configuration */}
                    <div className="settings-card">
                        <div className="settings-section-title">Emulator Configuration</div>
                        <div className="input-group">
                            <label>RetroArch Executable</label>
                            <div>
                                <FocusableInput
                                    className="input"
                                    value={form.raPath}
                                    readOnly
                                    placeholder="Not configured"
                                    focusKey="ra-path-input"
                                    onArrowPress={handleTopArrowPress}
                                />
                                <FocusableButton
                                    focusKey="browse-ra-button"
                                    className={`btn ${isSaving ? 'disabled' : ''}`}
                                    onClick={handleBrowseRA}
                                    onEnterPress={handleBrowseRA}
                                    onArrowPress={handleTopArrowPress}
                                    disabled={isSaving}
                                    onMouseEnter={() => getMouseActive() && !isSaving && setFocus('browse-ra-button')}
                                >
                                    Browse
                                </FocusableButton>
                            </div>
                        </div>
                    </div>

                    {/* Library Configuration */}
                    <div className="settings-card">
                        <div className="settings-section-title">Library Configuration</div>
                        <div className="input-group">
                            <label>Local ROM Library Path</label>
                            <div>
                                <FocusableInput
                                    className="input"
                                    value={form.libPath}
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

                    {/* Maintenance */}
                    <div className="settings-card">
                        <div className="settings-section-title">Maintenance</div>
                        <div className="input-group">
                            <label>System Maintenance</label>
                            <div className="button-row">
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
                                <FocusableButton
                                    focusKey="update-cores-button"
                                    className={`btn ${isSaving || isUpdatingCores ? 'disabled' : ''}`}
                                    onClick={handleUpdateCores}
                                    onEnterPress={handleUpdateCores}
                                    disabled={isSaving || isUpdatingCores}
                                    onMouseEnter={() => getMouseActive() && !isSaving && !isUpdatingCores && setFocus('update-cores-button')}
                                >
                                    Update Cores
                                </FocusableButton>
                                <FocusableButton
                                    focusKey="update-bios-button"
                                    className={`btn ${isSaving || isUpdatingBios ? 'disabled' : ''}`}
                                    onClick={handleUpdateBios}
                                    onEnterPress={handleUpdateBios}
                                    disabled={isSaving || isUpdatingBios}
                                    onMouseEnter={() => getMouseActive() && !isSaving && !isUpdatingBios && setFocus('update-bios-button')}
                                >
                                    Update BIOS
                                </FocusableButton>
                            </div>
                        </div>
                    </div>

                    {/* Offline Mode */}
                    <div className="settings-card">
                        <div className="settings-section-title">Offline Mode</div>
                        <div className="input-group">
                            <label>Sync Status</label>
                            <div className="button-row">
                                <FocusableButton
                                    focusKey="toggle-offline-button"
                                    className={`btn ${isSaving ? 'disabled' : ''}`}
                                    onClick={handleToggleOffline}
                                    onEnterPress={handleToggleOffline}
                                    disabled={isSaving}
                                    onMouseEnter={() => getMouseActive() && !isSaving && setFocus('toggle-offline-button')}
                                >
                                    {form.offlineMode ? "Enable Online" : "Enable Offline"}
                                </FocusableButton>
                                <FocusableButton
                                    focusKey="sync-metadata-button"
                                    className={`btn ${isSaving || isSyncing ? 'disabled' : ''}`}
                                    onClick={handleSyncMetadata}
                                    onEnterPress={handleSyncMetadata}
                                    disabled={isSaving || isSyncing}
                                    onMouseEnter={() => getMouseActive() && !isSyncing && setFocus('sync-metadata-button')}
                                >
                                    {isSyncing ? "Syncing..." : "Sync Metadata"}
                                </FocusableButton>
                            </div>
                        </div>
                    </div>

                    {/* RetroAchievements */}
                    <div className="settings-card">
                        <div className="settings-section-title">RetroAchievements</div>
                        <div className="input-group">
                            <label htmlFor="cheevosUser">Username</label>
                            <FocusableInput
                                id="cheevosUser"
                                focusKey="cheevos-user-input"
                                className="input"
                                value={form.cheevosUser}
                                onChange={(e) => setForm(prev => ({ ...prev, cheevosUser: e.target.value }))}
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
                                value={form.cheevosPass}
                                onChange={(e) => setForm(prev => ({ ...prev, cheevosPass: e.target.value }))}
                                autoComplete="off"
                            />
                        </div>
                    </div>

                    {/* RomM Connection */}
                    <div className="settings-card">
                        <div className="settings-section-title">RomM Connection</div>
                        <div className="input-group">
                            <label htmlFor="clientToken">Client Token</label>
                            <FocusableInput
                                id="clientToken"
                                focusKey="client-token-input"
                                className="input"
                                value={form.clientToken}
                                onChange={(e) => setForm(prev => ({ ...prev, clientToken: e.target.value }))}
                                autoComplete="off"
                                placeholder="rmm_..."
                            />
                            <div className="input-help-text" style={{ fontSize: '0.8rem', opacity: 0.7, marginTop: '0.5rem' }}>
                                A persistent token for stable connection. The app can auto-generate this if you login normally, or you can paste one from RomM Settings.
                            </div>
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
                    <LegendItem buttonAction="east" keyLabel="ESC" label="Back" />
                    <LegendItem buttonAction="south" keyLabel="ENTER" label="OK" />
                </div>
            </div>
        </div>
    );
}

export default Settings;
