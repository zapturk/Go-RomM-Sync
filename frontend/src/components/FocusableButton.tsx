import { useEffect } from 'react';
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';
import { getMouseActive } from '../inputMode';

interface FocusableButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
    focusKey?: string;
    onEnterPress?: () => void;
    onArrowPress?: (direction: string) => boolean;
    onFocus?: () => void;
    onBlur?: () => void;
}

export function FocusableButton({
    focusKey,
    onEnterPress,
    onArrowPress,
    onFocus,
    onBlur,
    children,
    className,
    ...props
}: FocusableButtonProps) {
    const { ref, focused } = useFocusable({
        focusKey,
        onEnterPress,
        onArrowPress,
        onFocus,
        onBlur,
    });

    useEffect(() => {
        if (focused && ref.current && !getMouseActive()) {
            const element = ref.current;
            // Use setTimeout to ensure focus state is fully applied before scrolling
            setTimeout(() => {
                element.scrollIntoView({
                    behavior: 'smooth',
                    block: 'center',
                    inline: 'nearest'
                });
            }, 50);
        }
    }, [focused]);

    return (
        <button
            ref={ref}
            className={`${className || ''} ${focused ? 'focused' : ''}`}
            {...props}
        >
            {children}
        </button>
    );
}
