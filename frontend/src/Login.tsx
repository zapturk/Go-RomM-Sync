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
        });
    }, []);

    // Auto-focus first input
    useEffect(() => {
        setTimeout(() => {
            setFocus('server-input');
        }, 100);
    }, []);

    function handleConnect() {
        if (!server || !username || !password) {
            setResultText("Please fill in all server details");
            return;
        }

        setIsLoggingIn(true);
        setResultText("Saving configuration...");

        const config = new types.AppConfig({
            romm_host: server,
            username: username,
            password: password
        });

        // First save the config (backend handles merging)
        SaveConfig(config)
            .then((saveResult) => {
                setResultText("Connecting to server...");
                console.log(saveResult);
                return RommLogin();
            })
            .then((token) => {
                setResultText("Success! Connected.");
                console.log("Logged in with token:", token);
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
        <div id="login-page" ref={ref} style={{ padding: '2rem 4rem', textAlign: 'left', maxWidth: '900px', margin: '0 auto', height: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="nav-header" style={{ marginBottom: '2.5rem', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '1rem' }}>
                <a
                    href="https://github.com/rommapp/romm"
                    target="_blank"
                    rel="noopener noreferrer"
                    title="Visit official RomM GitHub"
                    style={{ transition: 'transform 0.2s' }}
                    onMouseEnter={(e) => e.currentTarget.style.transform = 'scale(1.1)'}
                    onMouseLeave={(e) => e.currentTarget.style.transform = 'scale(1)'}
                >
                    <img
                        src="https://raw.githubusercontent.com/rommapp/romm/master/.github/resources/isotipo.svg"
                        alt="RomM Logo"
                        style={{ height: '80px', width: 'auto' }}
                    />
                </a>
                <h1 style={{ margin: 0, fontSize: '3rem', letterSpacing: '2px', textTransform: 'uppercase' }}>Login</h1>
            </div>

            <div id="result" className="result" style={{ marginBottom: '3rem', textAlign: 'center', color: 'rgba(255,255,255,0.7)', fontSize: '1.2rem', minHeight: '1.4em' }}>{resultText}</div>

            <div className="login-container" style={{ display: 'flex', flexDirection: 'column', gap: '2rem', alignItems: 'center' }}>
                <div className="section" style={{ width: '100%', maxWidth: '500px' }}>
                    <div className="input-group" style={{ width: '100%', marginBottom: '1.5rem' }}>
                        <label htmlFor="server">RomM Server URL</label>
                        <input
                            id="server"
                            className="input"
                            style={{ width: '100%' }}
                            value={server}
                            onChange={(e) => setServer(e.target.value)}
                            onKeyDown={handleInputKeyDown}
                            autoComplete="off"
                            placeholder="http://localhost:8080"
                            onFocus={() => setFocus('server-input')}
                        />
                    </div>
                    <div className="input-group" style={{ width: '100%', marginBottom: '1.5rem' }}>
                        <label htmlFor="username">Username</label>
                        <input
                            id="username"
                            className="input"
                            style={{ width: '100%' }}
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            onKeyDown={handleInputKeyDown}
                            autoComplete="off"
                        />
                    </div>
                    <div className="input-group" style={{ width: '100%', marginBottom: '2rem' }}>
                        <label htmlFor="password">Password</label>
                        <input
                            id="password"
                            className="input"
                            type="password"
                            style={{ width: '100%' }}
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            onKeyDown={handleInputKeyDown}
                            autoComplete="off"
                        />
                    </div>
                </div>

                <div style={{ width: '100%', maxWidth: '500px' }}>
                    <button
                        ref={connectRef}
                        className={`btn play-btn ${connectFocused ? 'focused' : ''}`}
                        style={{
                            width: '100%',
                            height: '60px',
                            fontSize: '1.3rem',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            margin: 0,
                            letterSpacing: '1px'
                        }}
                        onClick={handleConnect}
                        disabled={isLoggingIn}
                        onMouseEnter={() => getMouseActive() && setFocus('connect-button')}
                    >
                        {isLoggingIn ? "Connecting..." : "Connect"}
                    </button>
                </div>
            </div>

            <div className="branding-disclaimer" style={{
                marginTop: 'auto',
                padding: '2rem 0',
                textAlign: 'center',
                fontSize: '0.85rem',
                color: 'rgba(255,255,255,0.4)',
                maxWidth: '600px',
                alignSelf: 'center',
                lineHeight: '1.4'
            }}>
                This project is not affiliated with, endorsed by, or in any way officially connected with the
                <a
                    href="https://github.com/rommapp/romm"
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ color: 'inherit', textDecoration: 'underline', marginLeft: '4px' }}
                >
                    RomM project
                </a>.
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
        </div >
    );
}

export default Login;
