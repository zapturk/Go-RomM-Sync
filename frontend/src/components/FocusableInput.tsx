import { useEffect } from 'react';
import { useFocusable } from '@noriginmedia/norigin-spatial-navigation';

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
            // Use setTimeout to ensure focus state is fully applied before scrolling
            setTimeout(() => {
                element.scrollIntoView({
                    behavior: 'smooth',
                    block: 'center',
                    inline: 'nearest'
                });
                // Also focus the actual DOM element so user can start typing
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
