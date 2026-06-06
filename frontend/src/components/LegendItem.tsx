import React from 'react';

interface LegendItemProps {
    buttonAction?: 'north' | 'east' | 'south' | 'west';
    keyLabel: string;
    label: string;
}

export const LegendItem = ({ buttonAction, keyLabel, label }: LegendItemProps) => {
    return (
        <div className="legend-item">
            {buttonAction && (
                <div className="btn-icon show-gamepad">
                    <div className={`btn-dot north ${buttonAction === 'north' ? 'active' : ''}`} />
                    <div className={`btn-dot east ${buttonAction === 'east' ? 'active' : ''}`} />
                    <div className={`btn-dot south ${buttonAction === 'south' ? 'active' : ''}`} />
                    <div className={`btn-dot west ${buttonAction === 'west' ? 'active' : ''}`} />
                </div>
            )}
            <div className="key-icon show-keyboard">{keyLabel}</div>
            <span>{label}</span>
        </div>
    );
};
