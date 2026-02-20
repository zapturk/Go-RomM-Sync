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

    // UI state
    const [isLoggingIn, setIsLoggingIn] = useState(false);

    useEffect(() => {
        GetConfig().then((config) => {
            if (config.romm_host) setServer(config.romm_host);
            if (config.username) setUsername(config.username);
            if (config.password) setPassword(config.password);
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
            password: password
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
                <button className="btn" onClick={handleConnect} disabled={isLoggingIn}>
                    {isLoggingIn ? "Connecting..." : "Connect"}
                </button>
            </div>
        </div>
    )
}

export default Login
