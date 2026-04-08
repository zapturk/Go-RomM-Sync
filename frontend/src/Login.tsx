import { useState, useEffect } from 'react';
import { GetConfig, SaveConfig, Login as RommLogin } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from './inputMode';

interface LoginProps {
    onLoginSuccess: () => void;
}

function Login({ onLoginSuccess }: LoginProps) {
    const [resultText, setResultText] = useState("Connect to your RomM server to begin");
    const [server, setServer] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [clientToken, setClientToken] = useState('');

    // UI state
    const [isLoggingIn, setIsLoggingIn] = useState(false);

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    useEffect(() => {
        GetConfig().then((config) => {
            if (config.romm_host) setServer(config.romm_host);
            if (config.username) setUsername(config.username);
            if (config.password) setPassword(config.password);
            if (config.client_token) setClientToken(config.client_token);
        });
    }, []);

    // Auto-focus first input
    useEffect(() => {
        setTimeout(() => {
            setFocus('server-input');
        }, 100);
    }, []);

    function handleConnect() {
        if (!server || (!clientToken && (!username || !password))) {
            setResultText("Please enter server URL and either login credentials or a client token");
            return;
        }

        setIsLoggingIn(true);
        setResultText("Saving configuration...");

        const config = new types.AppConfig({
            romm_host: server,
            username: username,
            password: password,
            client_token: clientToken
        });

        // First save the config (backend handles merging)
        SaveConfig(config)
            .then((saveResult) => {
                setResultText("Connecting to server...");
                return RommLogin();
            })
            .then((token) => {
                setResultText("Success! Connected.");
                // Delay success slightly for better UX
                setTimeout(onLoginSuccess, 500);
            })
            .catch((err) => {
                setResultText("Error: " + err);
                console.error("Login failed:", err);
            })
            .finally(() => {
                setIsLoggingIn(false);
            });
    }

    const handleInputKeyDown = (e: React.KeyboardEvent) => {
        if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) {
            e.stopPropagation();
        }
    };

    const { ref: connectRef, focused: connectFocused } = useFocusable({
        focusKey: 'connect-button',
        onEnterPress: handleConnect
    });

    return (
        <div id="login-page" ref={ref}>
            <div className="login-content settings-content">
                <div className="settings-inner">
                    <div className="settings-header">
                        <a
                            href="https://github.com/rommapp/romm"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="logo-link"
                        >
                            <img
                                src="https://raw.githubusercontent.com/rommapp/romm/master/.github/resources/isotipo.svg"
                                alt="RomM Logo"
                                style={{ height: '80px', width: 'auto', marginBottom: '1rem' }}
                            />
                        </a>
                        <h1>Login</h1>
                        <div className="settings-status-box">{resultText}</div>
                    </div>

                    <div className="premium-card" style={{ maxWidth: '500px', margin: '0 auto', width: '100%' }}>
                        <div className="input-group">
                            <label htmlFor="server">RomM Server URL</label>
                            <input
                                id="server"
                                className="input"
                                value={server}
                                onChange={(e) => setServer(e.target.value)}
                                onKeyDown={handleInputKeyDown}
                                autoComplete="off"
                                placeholder="e.g. http://localhost:8080"
                                onFocus={() => setFocus('server-input')}
                            />
                        </div>
                        <div className="input-group">
                            <label htmlFor="username">Username</label>
                            <input
                                id="username"
                                className="input"
                                value={username}
                                onChange={(e) => setUsername(e.target.value)}
                                onKeyDown={handleInputKeyDown}
                                autoComplete="off"
                            />
                        </div>
                        <div className="input-group">
                            <label htmlFor="password">Password</label>
                            <input
                                id="password"
                                className="input"
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                onKeyDown={handleInputKeyDown}
                                autoComplete="off"
                            />
                        </div>

                        <div className="input-divider" style={{ textAlign: 'center', margin: '1rem 0', opacity: 0.5 }}>- OR -</div>

                        <div className="input-group">
                            <label htmlFor="token">Client Token (Persistent)</label>
                            <input
                                id="token"
                                className="input"
                                value={clientToken}
                                onChange={(e) => setClientToken(e.target.value)}
                                onKeyDown={handleInputKeyDown}
                                autoComplete="off"
                                placeholder="rmm_..."
                            />
                            <div className="input-help-text" style={{ fontSize: '0.8rem', opacity: 0.7, marginTop: '0.5rem' }}>
                                Found in RomM Settings → Client Tokens. This provides a more stable connection.
                            </div>
                        </div>

                        <button
                            ref={connectRef}
                            className={`btn play-btn ${connectFocused ? 'focused' : ''}`}
                            style={{
                                width: '100%',
                                height: '56px',
                                fontSize: '1.2rem',
                                marginTop: '1rem'
                            }}
                            onClick={handleConnect}
                            disabled={isLoggingIn}
                            onMouseEnter={() => getMouseActive() && setFocus('connect-button')}
                        >
                            {isLoggingIn ? "Connecting..." : "Connect"}
                        </button>
                    </div>

                    <div className="branding-disclaimer">
                        This project is not affiliated with, endorsed by, or in any way officially connected with the
                        <a
                            href="https://github.com/rommapp/romm"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            RomM project
                        </a>.
                    </div>
                </div>
            </div>

            <div className="input-legend" style={{ position: 'fixed', bottom: 0, left: 0, right: 0 }}>
                <div className="footer-left">
                    <span>{resultText}</span>
                </div>
                <div className="footer-right">
                    <div className="legend-item">
                        <div className="btn-icon show-gamepad">
                            <div className="btn-dot north"></div>
                            <div className="btn-dot east"></div>
                            <div className="btn-dot south active"></div>
                            <div className="btn-dot west"></div>
                        </div>
                        <div className="key-icon show-keyboard">ENTER</div>
                        <span>Login</span>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default Login;
