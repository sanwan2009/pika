import React, { type ReactElement, useMemo, useState } from 'react';
import {App, Button, Card, Space, Typography} from 'antd';
import { CopyIcon } from 'lucide-react';
import copy from 'copy-to-clipboard';
import linuxPng from '../../assets/os/linux.png';
import applePng from '../../assets/os/apple.png';
import windowsPng from '../../assets/os/win11.png';
import {
    AgentInstallLayout,
    ApiChooser,
    ConfigHelper,
    ServiceHelper,
    AGENT_NAME,
    AGENT_NAME_EXE,
} from './AgentInstallShared';
import { useAgentInstallConfig } from './useAgentInstallConfig';

const { Text } = Typography;

type OSType = 'linux-amd64' | 'linux-arm64' | 'linux-loong64' | 'darwin-amd64' | 'darwin-arm64' | 'windows-amd64' | 'windows-arm64';

interface OSConfig {
    name: string;
    icon: ReactElement;
    downloadUrl: string;
}

interface InstallStep {
    title: string;
    command: string;
}

const DEFAULT_OS: OSType = 'linux-amd64';

const AgentInstallManual = () => {
    const { message } = App.useApp();
    const [selectedOS, setSelectedOS] = useState<OSType>(DEFAULT_OS);
    const {
        apiKeys,
        selectedApiKey,
        selectedApiKeyId,
        setSelectedApiKeyId,
        loading,
        backendServerUrl,
        apiKeyOptions,
        refetchApiKeys,
    } = useAgentInstallConfig();

    const osConfigs: Record<OSType, OSConfig> = useMemo(() => ({
        'linux-amd64': {
            name: 'Linux (amd64)',
            icon: <img src={linuxPng} alt="Linux" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-linux-amd64',
        },
        'linux-arm64': {
            name: 'Linux (arm64)',
            icon: <img src={linuxPng} alt="Linux" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-linux-arm64',
        },
        'linux-loong64': {
            name: 'Linux (loongarch64)',
            icon: <img src={linuxPng} alt="Linux" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-linux-loong64',
        },
        'darwin-amd64': {
            name: 'macOS (amd64)',
            icon: <img src={applePng} alt="macOS" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-darwin-amd64',
        },
        'darwin-arm64': {
            name: 'macOS (arm64)',
            icon: <img src={applePng} alt="macOS" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-darwin-arm64',
        },
        'windows-amd64': {
            name: 'Windows (amd64)',
            icon: <img src={windowsPng} alt="Windows" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-windows-amd64.exe',
        },
        'windows-arm64': {
            name: 'Windows (arm64)',
            icon: <img src={windowsPng} alt="Windows" className="h-4 w-4" />,
            downloadUrl: '/api/agent/downloads/agent-windows-arm64.exe',
        },
    }), []);

    const copyToClipboard = (text: string) => {
        copy(text);
        message.success('已复制到剪贴板');
    };

    const getManualInstallSteps = (os: OSType): InstallStep[] => {
        const config = osConfigs[os];

        if (os.startsWith('windows')) {
            return [
                {
                    title: '1. 下载探针',
                    command: `# 使用 PowerShell 下载
Invoke-WebRequest -Uri "${backendServerUrl}${config.downloadUrl}?key=${selectedApiKey}" -OutFile "${AGENT_NAME_EXE}"

# 或者使用浏览器直接下载
# ${backendServerUrl}${config.downloadUrl}?key=${selectedApiKey}`
                },
                {
                    title: '2. 注册探针',
                    command: `.\\${AGENT_NAME_EXE} register --endpoint "${backendServerUrl}" --token "${selectedApiKey}"`
                },
                {
                    title: '3. 验证安装',
                    command: `.\\${AGENT_NAME_EXE} status`
                }
            ];
        }

        return [
            {
                title: '1. 下载探针',
                command: `# 使用 wget 下载
wget "${backendServerUrl}${config.downloadUrl}?key=${selectedApiKey}" -O ${AGENT_NAME}

# 或使用 curl 下载
curl -L "${backendServerUrl}${config.downloadUrl}?key=${selectedApiKey}" -o ${AGENT_NAME}`
            },
            {
                title: '2. 赋予执行权限',
                command: `chmod +x ${AGENT_NAME}`
            },
            {
                title: '3. 移动到系统路径',
                command: `sudo mv ${AGENT_NAME} /usr/local/bin/${AGENT_NAME}`
            },
            {
                title: '4. 注册探针',
                command: `sudo ${AGENT_NAME} register --endpoint "${backendServerUrl}" --token "${selectedApiKey}"`
            },
            {
                title: '5. 验证安装',
                command: `sudo ${AGENT_NAME} status`
            }
        ];
    };

    const selectedOSConfig = osConfigs[selectedOS];
    const selectedOSSteps = getManualInstallSteps(selectedOS);

    return (
        <AgentInstallLayout activeKey="manual">
            <Space orientation={'vertical'} className={'w-full'}>
                <ApiChooser
                    apiKeys={apiKeys}
                    selectedApiKey={selectedApiKeyId}
                    apiKeyOptions={apiKeyOptions}
                    loading={loading}
                    onSelectApiKey={setSelectedApiKeyId}
                    onApiKeyCreated={(apiKey) => {
                        void refetchApiKeys();
                        setSelectedApiKeyId(apiKey.id);
                    }}
                />
                <Card title="选择系统与架构">
                    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
                        {Object.entries(osConfigs).map(([key, config]) => {
                            const active = key === selectedOS;
                            return (
                                <button
                                    key={key}
                                    type="button"
                                    onClick={() => setSelectedOS(key as OSType)}
                                    className={active
                                        ? 'flex min-h-11 items-center gap-2 rounded-lg border border-[#1677ff] bg-[#eaf2ff] px-3 text-left text-[13px] font-semibold text-[#145dcc] dark:border-[#4086e8] dark:bg-[#1677ff]/15 dark:text-[#75adff]'
                                        : 'flex min-h-11 items-center gap-2 rounded-lg border border-[#d9dee7] bg-transparent px-3 text-left text-[13px] text-[#4e5969] hover:border-[#8dbbfa] hover:bg-[#f7faff] dark:border-[#30343d] dark:text-[#b7bcc5] dark:hover:border-[#42689c] dark:hover:bg-[#20242c]'}
                                >
                                    {config.icon}
                                    <span>{config.name}</span>
                                </button>
                            );
                        })}
                    </div>
                </Card>

                <Card title={`${selectedOSConfig.name} 安装步骤`}>
                    <div className="space-y-5">
                        {selectedOSSteps.map((step, index) => (
                            <div
                                key={step.title}
                                className="grid gap-3 border-b border-slate-200 pb-5 last:border-0 last:pb-0 dark:border-slate-700 lg:grid-cols-[150px_minmax(0,1fr)]"
                            >
                                <div className="flex items-start gap-2">
                                    <span className="grid size-6 shrink-0 place-items-center rounded-full bg-[#eaf2ff] text-xs font-semibold text-[#145dcc] dark:bg-[#1677ff]/15 dark:text-[#75adff]">
                                        {index + 1}
                                    </span>
                                    <Text strong className="pt-0.5 text-gray-900 dark:text-slate-100">
                                        {step.title.replace(/^\d+\.\s*/, '')}
                                    </Text>
                                </div>
                                <div className="min-w-0">
                                    <pre className="m-0 overflow-auto rounded-lg border border-slate-200 bg-slate-950 p-4 text-[13px] leading-6 text-slate-100 dark:border-slate-700">
                                        <code>{step.command}</code>
                                    </pre>
                                    <Button
                                        type="link"
                                        onClick={() => void copyToClipboard(step.command)}
                                        icon={<CopyIcon className="h-4 w-4"/>}
                                        size="small"
                                        style={{margin: '8px 0 0', padding: 0}}
                                        disabled={!selectedApiKey}
                                    >
                                        复制
                                    </Button>
                                </div>
                            </div>
                        ))}
                    </div>
                </Card>

                <div className="grid items-start gap-4 xl:grid-cols-2">
                    <ServiceHelper os={selectedOS}/>
                    <ConfigHelper/>
                </div>
            </Space>
        </AgentInstallLayout>
    );
};

export default AgentInstallManual;
