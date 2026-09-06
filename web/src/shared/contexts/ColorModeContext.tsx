import {createContext, useContext, useEffect, useState, type ReactNode} from 'react';
import {useRuntimeConfig} from '@/api/runtime';

export type ColorMode = 'light' | 'dark' | 'system';
export type ResolvedColorMode = 'light' | 'dark';

interface ColorModeContextValue {
    colorMode: ColorMode;
    resolvedColorMode: ResolvedColorMode;
    setColorMode: (mode: ColorMode) => void;
}

const ColorModeContext = createContext<ColorModeContextValue | undefined>(undefined);

export const ColorModeProvider = ({children}: {children: ReactNode}) => {
    const {data: runtime} = useRuntimeConfig();
    const [colorMode, setColorModeState] = useState<ColorMode>(() => {
        const saved = localStorage.getItem('colorMode') || localStorage.getItem('theme');
        if (saved === 'auto') return 'system';
        if (saved === 'light' || saved === 'dark' || saved === 'system') return saved;
        return 'system';
    });

    // 运行时配置加载后，如果用户没有本地偏好，使用服务端默认值
    useEffect(() => {
        if (!runtime) return;
        const saved = localStorage.getItem('colorMode') || localStorage.getItem('theme');
        if (!saved) {
            setColorModeState(runtime.system.defaultColorMode);
        }
    }, [runtime]);
    const [resolvedColorMode, setResolvedColorMode] = useState<ResolvedColorMode>('dark');

    const setColorMode = (mode: ColorMode) => {
        setColorModeState(mode);
        localStorage.setItem('colorMode', mode);
        localStorage.removeItem('theme');
    };

    useEffect(() => {
        const apply = () => {
            const resolved = colorMode === 'system'
                ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
                : colorMode;
            setResolvedColorMode(resolved);
            document.documentElement.classList.toggle('dark', resolved === 'dark');
            document.documentElement.dataset.colorMode = resolved;
        };
        apply();
        if (colorMode !== 'system') return;
        const media = window.matchMedia('(prefers-color-scheme: dark)');
        media.addEventListener('change', apply);
        return () => media.removeEventListener('change', apply);
    }, [colorMode]);

    return (
        <ColorModeContext.Provider value={{colorMode, resolvedColorMode, setColorMode}}>
            {children}
        </ColorModeContext.Provider>
    );
};

export const useColorMode = () => {
    const value = useContext(ColorModeContext);
    if (!value) throw new Error('useColorMode must be used within ColorModeProvider');
    return value;
};
