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
    const [isPlaying, setIsPlaying] = useState(false);

    useGamepad();

    useEffect(() => {
        init();

        // @ts-ignore
        if (window.runtime) {
            // @ts-ignore
            window.runtime.EventsOn("game-started", () => setIsPlaying(true));
            // @ts-ignore
            window.runtime.EventsOn("game-exited", () => setIsPlaying(false));
        }

        return () => {
            // @ts-ignore
            if (window.runtime) {
                // @ts-ignore
                window.runtime.EventsOff("game-started");
                // @ts-ignore
                window.runtime.EventsOff("game-exited");
            }
        };
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
        if (!isLoggedIn || isPlaying) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.code === 'Escape') {
                if (view === 'settings') {
                    e.preventDefault();
                    setView('library');
                }
            }
        };
        // Use capture phase so we can stop propagation before norigin-spatial-navigation catches it
        const captureKeyDown = (e: KeyboardEvent) => {
            if (isPlaying) {
                e.stopPropagation();
                e.preventDefault();
                return;
            }
        };

        window.addEventListener('keydown', handleKeyDown);
        window.addEventListener('keydown', captureKeyDown, true);
        return () => {
            window.removeEventListener('keydown', handleKeyDown);
            window.removeEventListener('keydown', captureKeyDown, true);
        };
    }, [isLoggedIn, view, isPlaying]);

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
                <>
                    <div className={view === 'settings' ? '' : 'hidden-view'}>
                        <Settings />
                    </div>
                    <div className={view === 'library' ? '' : 'hidden-view'}>
                        <Library
                            onOpenSettings={() => setView('settings')}
                            isActive={view === 'library'}
                        />
                    </div>
                </>
            )}
        </div>
    );
}

export default App;
