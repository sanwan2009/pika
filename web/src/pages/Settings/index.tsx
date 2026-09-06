import type {ReactNode} from 'react';
import {Bell, MessageSquare, Palette, Settings2, Wifi} from 'lucide-react';
import type {LucideIcon} from 'lucide-react';
import AlertSettings from './AlertSettings';
import NotificationChannels from './NotificationChannels';
import SystemConfig from './SystemConfig';
import PublicIPConfig from './PublicIPConfig';
import ThemeSettings from './ThemeSettings';
import {PageHeader} from '@/components/PageHeader';
import {PagePanel} from '@/components/PagePanel';
import {useSearchParams} from 'react-router-dom';
import {cn} from '@/lib/utils';

// 默认 IPv4 API 列表
export const defaultIPv4APIs = [
    'https://myip.ipip.net',
    'https://ddns.oray.com/checkip',
    'https://ip.3322.net',
    'https://4.ipw.cn',
    'https://v4.yinghualuo.cn/bejson',
];

// 默认 IPv6 API 列表
export const defaultIPv6APIs = [
    'https://speed.neu6.edu.cn/getIP.php',
    'https://v6.ident.me',
    'https://6.ipw.cn',
    'https://v6.yinghualuo.cn/bejson',
];

interface SettingsTab {
    key: string;
    label: string;
    icon: LucideIcon;
    content: ReactNode;
}

const settingsTabs: SettingsTab[] = [
    {
        key: 'system',
        label: '系统配置',
        icon: Settings2,
        content: <SystemConfig/>,
    },
    {
        key: 'themes',
        label: '主题管理',
        icon: Palette,
        content: <ThemeSettings/>,
    },
    {
        key: 'public-ip',
        label: '公网 IP 采集',
        icon: Wifi,
        content: (
            <PublicIPConfig
                defaultIPv4APIs={defaultIPv4APIs}
                defaultIPv6APIs={defaultIPv6APIs}
            />
        ),
    },
    {
        key: 'channels',
        label: '通知渠道',
        icon: MessageSquare,
        content: <NotificationChannels/>,
    },
    {
        key: 'alert',
        label: '告警规则',
        icon: Bell,
        content: <AlertSettings/>,
    },
];

const Settings = () => {
    const [searchParams, setSearchParams] = useSearchParams({tab: 'system'});

    const activeKey = settingsTabs.some((tab) => tab.key === searchParams.get('tab'))
        ? searchParams.get('tab')!
        : 'system';
    const activeTab = settingsTabs.find((tab) => tab.key === activeKey) ?? settingsTabs[0];

    return (
        <div className="flex min-w-0 flex-col gap-6">
            <PageHeader title="系统设置"/>

            <div className="grid grid-cols-[200px_minmax(0,1fr)] items-start gap-6 max-md:block">
                <nav
                    className="flex flex-col gap-1 rounded-xl bg-[#f5f6f8] p-1.5 lg:sticky lg:top-[76px] dark:bg-[#1c2028] max-md:mb-2 max-md:flex-row max-md:overflow-x-auto"
                    aria-label="设置分类"
                >
                    {settingsTabs.map((tab) => {
                        const Icon = tab.icon;
                        const active = tab.key === activeKey;
                        return (
                            <button
                                key={tab.key}
                                type="button"
                                className={cn(
                                    'flex min-h-10 w-full cursor-pointer items-center gap-2 rounded-[10px] border-0 px-2.5 py-2 text-left text-[13px] transition-colors max-md:w-auto max-md:shrink-0 max-md:whitespace-nowrap',
                                    active
                                        ? 'bg-[#eaf2ff] font-semibold text-[#145dcc] dark:bg-[#1677ff]/20 dark:text-[#75adff]'
                                        : 'bg-transparent font-medium text-[#1f2329] hover:bg-[#e8ebf0] dark:text-[#e6e8ec] dark:hover:bg-[#272b33]',
                                )}
                                onClick={() => setSearchParams({tab: tab.key})}
                                aria-current={active ? 'page' : undefined}
                            >
                                <Icon size={16} className="shrink-0"/>
                                <span className="truncate">{tab.label}</span>
                            </button>
                        );
                    })}
                </nav>

                <PagePanel className="min-w-0">
                    {activeTab.content}
                </PagePanel>
            </div>
        </div>
    );
};

export default Settings;
