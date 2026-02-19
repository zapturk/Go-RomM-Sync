import { useState, useEffect } from 'react';
import { GetConfig, SaveConfig } from "../wailsjs/go/main/App";
import { types } from "../wailsjs/go/models";
// App.css is imported in App.tsx so specific styles might work if not scoped, 
// but it's safer to not rely on App.tsx imports if we want this standalone.
// For now, since App.tsx usually imports global styles, we might assume App.css is global enough.
// or we can import it here too.

function Login() {
    const [resultText, setResultText] = useState("Please enter your RomM details 👇");
    const [server, setServer] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const updateResultText = (result: string) => setResultText(result);

    useEffect(() => {
        GetConfig().then((config) => {
            if (config.romm_host) setServer(config.romm_host);
            if (config.username) setUsername(config.username);
            if (config.password) setPassword(config.password);
        });
    }, []);

    function saveConfig() {
        const config = new types.AppConfig({
            romm_host: server,
            username: username,
            password: password
        });
        SaveConfig(config).then(updateResultText);
    }

    return (
        <div id="login">
            <div id="result" className="result">{resultText}</div>
            <div id="input" className="input-box">
                <div className="input-group">
                    <label htmlFor="server">RomM Server URL</label>
                    <input id="server" className="input" onChange={(e) => setServer(e.target.value)} value={server} autoComplete="off" name="server" type="text" placeholder="http://localhost:8080" />
                </div>
                <div className="input-group">
                    <label htmlFor="username">Username</label>
                    <input id="username" className="input" onChange={(e) => setUsername(e.target.value)} value={username} autoComplete="off" name="username" type="text" />
                </div>
                <div className="input-group">
                    <label htmlFor="password">Password</label>
                    <input id="password" className="input" onChange={(e) => setPassword(e.target.value)} value={password} autoComplete="off" name="password" type="password" />
                </div>
                <button className="btn" onClick={saveConfig}>Connect</button>
            </div>
        </div>
    )
}

export default Login
