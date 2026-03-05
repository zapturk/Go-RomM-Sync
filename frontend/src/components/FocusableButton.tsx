import { useEffect } from 'react';
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';

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
        if (focused && ref.current) {
            ref.current.scrollIntoView({
                behavior: 'smooth',
                block: 'center',
            });
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
