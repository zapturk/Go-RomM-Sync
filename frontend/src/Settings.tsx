import { useState, useEffect } from 'react';
import { GetConfig, SaveConfig, SelectRetroArchExecutable } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';

function Settings() {
    const [config, setConfig] = useState<types.AppConfig | null>(null);
    const [status, setStatus] = useState("Configure your application settings");
    const [isSaving, setIsSaving] = useState(false);

    // Form states
    const [raPath, setRaPath] = useState('');
    const [cheevosUser, setCheevosUser] = useState('');
    const [cheevosPass, setCheevosPass] = useState('');

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    useEffect(() => {
        GetConfig().then((cfg) => {
            console.log("Settings loaded config:", cfg);
            setConfig(cfg);
            setRaPath(cfg.retroarch_path || '');
            setCheevosUser(cfg.cheevos_username || '');
            setCheevosPass(cfg.cheevos_password || '');
        });
    }, []);

    const handleBrowseRA = () => {
        SelectRetroArchExecutable().then((path) => {
            if (path) {
                setRaPath(path);
                setStatus("RetroArch path updated.");
            }
        });
    };

    const handleSave = () => {
        if (!config) return;

        setIsSaving(true);
        setStatus("Saving settings...");

        // We only send the updated fields, the backend SaveConfig handles merging
        const updatedConfig = new types.AppConfig({
            retroarch_path: raPath,
            cheevos_username: cheevosUser,
            cheevos_password: cheevosPass
        });

        SaveConfig(updatedConfig)
            .then((res) => {
                setStatus("Settings saved successfully!");
                console.log(res);
            })
            .catch((err) => {
                setStatus("Error: " + err);
            })
            .finally(() => {
                setIsSaving(false);
            });
    };

    const handleInputKeyDown = (e: React.KeyboardEvent) => {
        if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) {
            e.stopPropagation();
        }
    };

    // Auto-focus browse button on load
    useEffect(() => {
        setTimeout(() => {
            setFocus('browse-button');
        }, 100);
    }, []);

    const { ref: browseRef, focused: browseFocused } = useFocusable({
        focusKey: 'browse-button',
        onEnterPress: handleBrowseRA
    });

    const { ref: saveRef, focused: saveFocused } = useFocusable({
        focusKey: 'save-button',
        onEnterPress: handleSave
    });

    if (!config) return <div className="loading-screen"><h2>Loading settings...</h2></div>;

    return (
        <div id="settings-page" ref={ref} style={{ padding: '2rem 4rem', textAlign: 'left', maxWidth: '900px', margin: '0 auto' }}>
            <div className="nav-header" style={{ marginBottom: '2.5rem', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <h1 style={{ margin: 0, fontSize: '2.5rem' }}>Settings</h1>
            </div>

            <div id="result" className="result" style={{ marginBottom: '2rem', textAlign: 'center', color: 'rgba(255,255,255,0.7)', fontSize: '1.1rem' }}>{status}</div>

            <div className="settings-container" style={{ display: 'flex', flexDirection: 'column', gap: '2rem', alignItems: 'center' }}>
                <div className="section" style={{ width: '100%', maxWidth: '500px' }}>
                    <h3 style={{ borderBottom: '1px solid rgba(255,255,255,0.1)', paddingBottom: '0.5rem', marginBottom: '1.5rem', color: 'rgba(255,255,255,0.9)' }}>Emulator Configuration</h3>
                    <div className="input-group" style={{ width: '100%' }}>
                        <label>RetroArch Executable</label>
                        <div style={{ display: 'flex', gap: '10px', width: '100%' }}>
                            <input
                                className="input"
                                value={raPath}
                                readOnly
                                style={{ flex: 1, backgroundColor: 'rgba(255,255,255,0.05)', color: 'rgba(255,255,255,0.5)' }}
                                placeholder="Not configured"
                            />
                            <button
                                ref={browseRef}
                                className={`btn ${browseFocused ? 'focused' : ''}`}
                                style={{ margin: 0, minWidth: '100px' }}
                                onClick={handleBrowseRA}
                                onMouseEnter={() => getMouseActive() && setFocus('browse-button')}
                            >
                                Browse
                            </button>
                        </div>
                    </div>
                </div>

                <div className="section" style={{ width: '100%', maxWidth: '500px' }}>
                    <h3 style={{ borderBottom: '1px solid rgba(255,255,255,0.1)', paddingBottom: '0.5rem', marginBottom: '1.5rem', color: 'rgba(255,255,255,0.9)' }}>RetroAchievements</h3>
                    <div className="input-group" style={{ width: '100%', marginBottom: '1rem' }}>
                        <label htmlFor="cheevosUser">Username</label>
                        <input
                            id="cheevosUser"
                            className="input"
                            style={{ width: '100%' }}
                            value={cheevosUser}
                            onChange={(e) => setCheevosUser(e.target.value)}
                            onKeyDown={handleInputKeyDown}
                            autoComplete="off"
                        />
                    </div>
                    <div className="input-group" style={{ width: '100%' }}>
                        <label htmlFor="cheevosPass">Password</label>
                        <input
                            id="cheevosPass"
                            className="input"
                            type="password"
                            style={{ width: '100%' }}
                            value={cheevosPass}
                            onChange={(e) => setCheevosPass(e.target.value)}
                            onKeyDown={handleInputKeyDown}
                            autoComplete="off"
                        />
                    </div>
                </div>

                <div style={{ marginTop: '2rem', width: '100%', maxWidth: '500px', display: 'flex', justifyContent: 'center' }}>
                    <button
                        ref={saveRef}
                        className={`btn play-btn ${saveFocused ? 'focused' : ''}`}
                        style={{
                            width: '100%',
                            height: '50px',
                            fontSize: '1.1rem',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            margin: 0
                        }}
                        onClick={handleSave}
                        disabled={isSaving}
                        onMouseEnter={() => getMouseActive() && setFocus('save-button')}
                    >
                        {isSaving ? "Saving..." : "Save Settings"}
                    </button>
                </div>
            </div>

            <div className="input-legend" style={{ position: 'fixed', bottom: 0, left: 0, right: 0 }}>
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
