import {useQuery} from '@tanstack/react-query';

export interface RuntimeConfig {
    apiVersion: string;
    system: {
        nameZh: string;
        nameEn: string;
        logo: string;
        icpCode: string;
        version: string;
        defaultView: string;
        defaultColorMode: 'light' | 'dark' | 'system';
    };
    theme: {id: string; version: string};
    features: Record<string, boolean>;
}

const fallback: RuntimeConfig = {
    apiVersion: 'v1',
    system: {
        nameZh: '皮卡监控',
        nameEn: 'Pika Monitor',
        logo: '/api/logo',
        icpCode: '',
        version: '',
        defaultView: 'grid',
        defaultColorMode: 'system',
    },
    theme: {id: 'default', version: ''},
    features: {},
};

/** 从 /api/config 获取运行时配置（系统名称、logo、版本等）。失败时返回 fallback。 */
export const useRuntimeConfig = () => {
    return useQuery({
        queryKey: ['runtime-config'],
        queryFn: async (): Promise<RuntimeConfig> => {
            const res = await fetch('/api/config');
            if (!res.ok) throw new Error('无法加载运行时配置');
            return res.json();
        },
        staleTime: 5 * 60 * 1000,
        retry: 1,
    });
};

export {fallback as defaultRuntimeConfig};
