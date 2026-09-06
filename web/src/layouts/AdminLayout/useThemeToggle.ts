import {type RefObject} from 'react';
import {flushSync} from 'react-dom';
import {useColorMode} from '@/shared/contexts/ColorModeContext';

/** 切换明暗模式，支持 View Transition API 的圆形扩散动画。 */
export function useThemeToggle(themeButtonRef: RefObject<HTMLButtonElement | null>) {
    const {resolvedColorMode: appliedTheme, setColorMode: setTheme} = useColorMode();

    const toggleTheme = async () => {
        const newTheme = appliedTheme === 'dark' ? 'light' : 'dark';

        if (
            !themeButtonRef.current ||
            !document.startViewTransition ||
            window.matchMedia('(prefers-reduced-motion: reduce)').matches
        ) {
            setTheme(newTheme);
            return;
        }

        await document.startViewTransition(() => {
            flushSync(() => setTheme(newTheme));
        }).ready;

        const {top, left, width, height} = themeButtonRef.current.getBoundingClientRect();
        const x = left + width / 2;
        const y = top + height / 2;
        const right = window.innerWidth - left;
        const bottom = window.innerHeight - top;
        const maxRadius = Math.hypot(Math.max(left, right), Math.max(top, bottom));

        document.documentElement.animate(
            {
                clipPath: [
                    `circle(0px at ${x}px ${y}px)`,
                    `circle(${maxRadius}px at ${x}px ${y}px)`,
                ],
            },
            {duration: 500, easing: 'ease-in-out', pseudoElement: '::view-transition-new(root)'},
        );
    };

    return {appliedTheme, toggleTheme};
}
