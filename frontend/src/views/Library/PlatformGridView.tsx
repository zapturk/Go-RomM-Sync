import { useRef, useEffect } from 'react';
import { types } from "../../../wailsjs/go/models";
import { PlatformCard } from "../../PlatformCard";
import { SettingsIcon } from "../../components/Icons";
import { getMouseActive } from '../../inputMode';
import { useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation';

interface PlatformGridViewProps {
    platforms: types.Platform[];
    isLoading: boolean;
    onSelectPlatform: (platform: types.Platform) => void;
    onOpenSettings: () => void;
    columns: number;
    syncTrigger: number;
    lastViewedPlatformId: number | null;
    gridRef: React.RefObject<HTMLDivElement>;
}

export function PlatformGridView({
    platforms,
    isLoading,
    onSelectPlatform,
    onOpenSettings,
    columns,
    syncTrigger,
    lastViewedPlatformId,
    gridRef
}: PlatformGridViewProps) {

    const { ref, focusKey } = useFocusable({
        trackChildren: true
    });

    const { ref: configRef, focused: configFocused, focusSelf: focusConfig } = useFocusable({
        focusKey: 'config-button',
        onEnterPress: onOpenSettings,
        onArrowPress: (direction: string) => {
            if (direction === 'up' || direction === 'left') {
                return false;
            }
            return true;
        }
    });

    useEffect(() => {
        if (platforms.length > 0) {
            setTimeout(() => {
                if (lastViewedPlatformId && platforms.some(p => p.id === lastViewedPlatformId)) {
                    setFocus(`platform-${lastViewedPlatformId}`);
                } else {
                    setFocus(`platform-${platforms[0].id}`);
                }
            }, 100);
        } else if (!isLoading) {
            setTimeout(() => {
                focusConfig();
            }, 100);
        }
    }, [platforms.length, lastViewedPlatformId, isLoading, focusConfig]);

    return (
        <div className="platform-grid-view" ref={ref}>
            <div className="nav-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', position: 'relative', minHeight: '60px' }}>
                <button
                    ref={configRef}
                    className={`btn config-btn ${configFocused ? 'focused' : ''} ${isLoading ? 'disabled' : ''}`}
                    title="Open Settings"
                    disabled={isLoading}
                    style={{
                        margin: 0,
                        padding: '5px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        background: 'transparent',
                        border: '1px solid rgba(255,255,255,0.1)',
                        position: 'absolute',
                        left: '40px'
                    }}
                    onMouseEnter={() => {
                        if (getMouseActive() && !isLoading) {
                            focusConfig();
                        }
                    }}
                    onClick={onOpenSettings}
                >
                    <SettingsIcon size={24} />
                </button>
                <h1 style={{ margin: 0 }}>Platforms</h1>
            </div>
            <div className="grid-container" ref={gridRef}>
                {platforms.map((platform, index) => (
                    <PlatformCard
                        key={platform.id}
                        platform={platform}
                        isLeftmost={index % columns === 0}
                        onClick={() => onSelectPlatform(platform)}
                        onEnterPress={() => onSelectPlatform(platform)}
                        syncTrigger={syncTrigger}
                    />
                ))}
            </div>
            {platforms.length === 0 && !isLoading && (
                <div className="empty-state-container">
                    <div className="empty-state-card">
                        <div className="empty-state-icon">🎮</div>
                        <h2>No platforms found</h2>
                        <p>Your library is empty. Please check your RomM host and local library path in settings to connect your collection.</p>
                        <button
                            className="btn play-btn"
                            style={{ marginTop: '1.5rem', width: 'auto', padding: '10px 30px' }}
                            onClick={onOpenSettings}
                        >
                            Open Settings
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
