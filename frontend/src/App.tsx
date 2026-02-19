import { useState, useEffect } from 'react';
import './App.css';
import LoginView from './Login';
import Library from './Library';
import { Login } from "../wailsjs/go/main/App";

function App() {
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        // Try to auto-login using saved config
        Login() // Calling Login with empty strings will trigger the backend to use saved config if available
            .then((token) => {
                if (token) {
                    console.log("Auto-login successful");
                    setIsLoggedIn(true);
                }
            })
            .catch((err) => {
                console.log("Auto-login failed or no config:", err);
            })
            .finally(() => {
                setIsLoading(false);
            });
    }, []);

    if (isLoading) {
        return (
            <div id="App" className="loading-screen">
                <h2>Loading...</h2>
            </div>
        );
    }

    return (
        <div id="App">
            {!isLoggedIn ? (
                <LoginView onLoginSuccess={() => setIsLoggedIn(true)} />
            ) : (
                <Library />
            )}
        </div>
    )
}

export default App
