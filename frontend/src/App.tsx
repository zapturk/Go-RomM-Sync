import { useState } from 'react';
import './App.css';
import Login from './Login';
import Library from './Library';

function App() {
    const [isLoggedIn, setIsLoggedIn] = useState(false);

    return (
        <div id="App">
            {!isLoggedIn ? (
                <Login onLoginSuccess={() => setIsLoggedIn(true)} />
            ) : (
                <Library />
            )}
        </div>
    )
}

export default App
