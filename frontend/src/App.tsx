import { useState, useEffect } from 'react';
import './App.css';
import LoginView from './Login';
import Library from './Library';
import Settings from './Settings';
import { Login } from "../wailsjs/go/main/App";
import { init } from '@noriginmedia/norigin-spatial-navigation';
import { useGamepad } from './useGamepad';
import './inputMode'; // Activate input mode tracking

function App() {
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [isLoading, setIsLoading] = useState(true);
    const [view, setView] = useState<'library' | 'settings'>('library');

    useGamepad();

    useEffect(() => {
        init();
    }, []);

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

    // Global back handling (only when logged in)
    useEffect(() => {
        if (!isLoggedIn) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.code === 'Backspace' || e.code === 'Escape') {
                if (view === 'settings') {
                    e.preventDefault();
                    setView('library');
                }
            }
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [isLoggedIn, view]);

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
            ) : view === 'settings' ? (
                <Settings />
            ) : (
                <Library onOpenSettings={() => setView('settings')} />
            )}
        </div>
    );
}

export default App;
