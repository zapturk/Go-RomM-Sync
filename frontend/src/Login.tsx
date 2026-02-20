import { useState, useEffect } from 'react';
import { GetConfig, SaveConfig, Login as RommLogin } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";

interface LoginProps {
    onLoginSuccess: () => void;
}

function Login({ onLoginSuccess }: LoginProps) {
    const [resultText, setResultText] = useState("Please enter your RomM details 👇");
    const [server, setServer] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [cheevosUsername, setCheevosUsername] = useState('');
    const [cheevosPassword, setCheevosPassword] = useState('');

    // UI state
    const [isLoggingIn, setIsLoggingIn] = useState(false);

    useEffect(() => {
        GetConfig().then((config) => {
            if (config.romm_host) setServer(config.romm_host);
            if (config.username) setUsername(config.username);
            if (config.password) setPassword(config.password);
            if (config.cheevos_username) setCheevosUsername(config.cheevos_username);
            if (config.cheevos_password) setCheevosPassword(config.cheevos_password);
        });
    }, []);

    function handleConnect() {
        if (!server || !username || !password) {
            setResultText("Please fill in all fields");
            return;
        }

        setIsLoggingIn(true);
        setResultText("Saving config and connecting...");

        const config = new types.AppConfig({
            romm_host: server,
            username: username,
            password: password,
            cheevos_username: cheevosUsername,
            cheevos_password: cheevosPassword
        });

        // First save the config
        SaveConfig(config)
            .then((saveResult) => {
                // If save is successful (or returns message), try to login
                console.log(saveResult);
                return RommLogin();
            })
            .then((token) => {
                setResultText("Success! Connected to RomM.");
                console.log("Logged in with token:", token);
                onLoginSuccess();
            })
            .catch((err) => {
                setResultText("Error: " + err);
                console.error("Login/Save failed:", err);
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

    return (
        <div id="login">
            <div id="result" className="result">{resultText}</div>
            <div id="input" className="input-box">
                <div className="input-group">
                    <label htmlFor="server">RomM Server URL</label>
                    <input id="server" className="input" onChange={(e) => setServer(e.target.value)} onKeyDown={handleInputKeyDown} value={server} autoComplete="off" name="server" type="text" placeholder="http://localhost:8080" />
                </div>
                <div className="input-group">
                    <label htmlFor="username">Username</label>
                    <input id="username" className="input" onChange={(e) => setUsername(e.target.value)} onKeyDown={handleInputKeyDown} value={username} autoComplete="off" name="username" type="text" />
                </div>
                <div className="input-group">
                    <label htmlFor="password">Password</label>
                    <input id="password" className="input" onChange={(e) => setPassword(e.target.value)} onKeyDown={handleInputKeyDown} value={password} autoComplete="off" name="password" type="password" />
                </div>

                <div className="section-title" style={{ marginTop: '20px', fontSize: '0.9rem', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase', letterSpacing: '1px' }}>RetroAchievements (Optional)</div>
                <div className="input-group">
                    <label htmlFor="cheevosUsername">RA Username</label>
                    <input id="cheevosUsername" className="input" onChange={(e) => setCheevosUsername(e.target.value)} onKeyDown={handleInputKeyDown} value={cheevosUsername} autoComplete="off" name="cheevosUsername" type="text" />
                </div>
                <div className="input-group">
                    <label htmlFor="cheevosPassword">RA Password</label>
                    <input id="cheevosPassword" className="input" onChange={(e) => setCheevosPassword(e.target.value)} onKeyDown={handleInputKeyDown} value={cheevosPassword} autoComplete="off" name="cheevosPassword" type="password" />
                </div>
                <button className="btn" onClick={handleConnect} disabled={isLoggingIn}>
                    {isLoggingIn ? "Connecting..." : "Connect"}
                </button>
            </div>
        </div>
    )
}

export default Login
