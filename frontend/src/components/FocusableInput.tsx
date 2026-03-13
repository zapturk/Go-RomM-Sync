import { useEffect } from 'react';
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from '../inputMode';

interface FocusableInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
    focusKey?: string;
    onEnterPress?: () => void;
    onArrowPress?: (direction: string) => boolean;
    onFocus?: () => void;
    onBlur?: () => void;
}

export function FocusableInput({
    focusKey,
    onEnterPress,
    onArrowPress,
    onFocus,
    onBlur,
    className,
    ...props
}: FocusableInputProps) {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress,
        onArrowPress,
        onFocus,
        onBlur,
    });

    useEffect(() => {
        if (focused && ref.current) {
            const element = ref.current;
            // If it's a mouse interaction, we focus the element so user can start typing
            // but we suppress the automatic scrollIntoView which can be jarring on hover
            if (getMouseActive()) {
                element.focus();
                return;
            }

            // For keyboard/gamepad, we want both focus and scroll
            setTimeout(() => {
                element.scrollIntoView({
                    behavior: 'smooth',
                    block: 'center',
                    inline: 'nearest'
                });
                element.focus();
            }, 50);
        }
    }, [focused]);

    return (
        <input
            ref={ref}
            className={`${className || ''} ${focused ? 'focused' : ''}`}
            {...props}
        />
    );
}
