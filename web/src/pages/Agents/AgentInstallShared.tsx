import React, { type ReactNode, useState } from 'react';
import { Alert, Button, Card, Input, Select, Typography } from 'antd';
import { Link, useNavigate } from 'react-router-dom';
import {ArrowLeft, Plus, TerminalSquare, Wrench} from 'lucide-react';
import type { ApiKey } from '@/types';
import {PageHeader} from '@/components/PageHeader';
import ApiKeyModal from '../ApiKeys/ApiKeyModal';

const { Paragraph, Text } = Typography;

export const AGENT_NAME = 'pika-agent';
export const AGENT_NAME_EXE = 'pika-agent.exe';
export const CONFIG_PATH = '~/.pika/agent.yaml';

type InstallNavKey = 'one-click' | 'manual';

export type ApiKeyOption = {
    label: string;
    value: string;
};

export type ApiChooserProps = {
    apiKeys: ApiKey[];
    selectedApiKey: string;
    apiKeyOptions: ApiKeyOption[];
    loading: boolean;
    onSelectApiKey: (value: string) => void;
    onApiKeyCreated?: (apiKey: ApiKey) => void;
};

export const AgentInstallLayout = ({ activeKey, children }: { activeKey: InstallNavKey; children: ReactNode }) => {
    const navigate = useNavigate();

    return (
        <div className="space-y-4">
            <PageHeader
                title="探针部署指南"
                actions={[{
                    key: 'back',
                    label: '返回探针列表',
                    icon: <ArrowLeft size={16}/>,
                    onClick: () => navigate('/admin/agents'),
                }]}
            />

            <div className="flex items-center gap-1 rounded-[10px] border border-[#e8ebf0] bg-white p-1 dark:border-[#272b33] dark:bg-[#171a21]">
                    <button
                        type="button"
                        onClick={() => navigate('/admin/agents-install/one-click')}
                        className={activeKey === 'one-click'
                            ? 'flex min-h-9 items-center gap-2 rounded-lg border-0 bg-[#eaf2ff] px-3 text-[13px] font-semibold text-[#145dcc] dark:bg-[#1677ff]/15 dark:text-[#75adff]'
                            : 'flex min-h-9 items-center gap-2 rounded-lg border-0 bg-transparent px-3 text-[13px] text-[#646a73] hover:bg-[#f5f6f8] dark:text-[#9ba1ab] dark:hover:bg-[#20242c]'}
                    >
                        <TerminalSquare size={16}/>
                        一键安装
                    </button>
                    <button
                        type="button"
                        onClick={() => navigate('/admin/agents-install/manual')}
                        className={activeKey === 'manual'
                            ? 'flex min-h-9 items-center gap-2 rounded-lg border-0 bg-[#eaf2ff] px-3 text-[13px] font-semibold text-[#145dcc] dark:bg-[#1677ff]/15 dark:text-[#75adff]'
                            : 'flex min-h-9 items-center gap-2 rounded-lg border-0 bg-transparent px-3 text-[13px] text-[#646a73] hover:bg-[#f5f6f8] dark:text-[#9ba1ab] dark:hover:bg-[#20242c]'}
                    >
                        <Wrench size={16}/>
                        手动安装
                    </button>
            </div>

            {children}
        </div>
    );
};

export const ApiChooser = ({
    apiKeys,
    selectedApiKey,
    apiKeyOptions,
    loading,
    onSelectApiKey,
    onApiKeyCreated,
}: ApiChooserProps) => {
    const [isCreateModalVisible, setIsCreateModalVisible] = useState(false);
    const [newApiKeyData, setNewApiKeyData] = useState<ApiKey | null>(null);
    const [showApiKeyModal, setShowApiKeyModal] = useState(false);
    const navigate = useNavigate();

    const handleCreateSuccess = (apiKey?: ApiKey) => {
        setIsCreateModalVisible(false);
        if (apiKey) {
            setNewApiKeyData(apiKey);
            setShowApiKeyModal(true);
            if (onApiKeyCreated) {
                onApiKeyCreated(apiKey);
            }
        }
    };

    return (
        <>
            <Card type="inner" title="配置选项">
                <div>
                    <div className="mb-1 flex items-center justify-between">
                        <span className="text-gray-600 dark:text-slate-400">选择通信密钥</span>
                        <Button
                            size="small"
                            type="primary"
                            icon={<Plus size={14}/>}
                            onClick={() => setIsCreateModalVisible(true)}
                        >
                            创建密钥
                        </Button>
                    </div>
                    {apiKeys.length === 0 ? (
                        <Alert
                            message="暂无可用的通信密钥"
                            description={
                                <span>
                                    请点击上方"创建密钥"按钮生成一个通信密钥，或前往 <Link to="/admin/api-keys">通信密钥管理</Link> 页面
                                </span>
                            }
                            type="warning"
                            showIcon
                            className="mt-2"
                        />
                    ) : (
                        <Select
                            className="w-full"
                            value={selectedApiKey}
                            onChange={onSelectApiKey}
                            options={apiKeyOptions}
                            loading={loading}
                            placeholder="请选择通信密钥"
                        />
                    )}
                </div>
            </Card>

            <ApiKeyModal
                open={isCreateModalVisible}
                apiKeyId={undefined}
                onCancel={() => setIsCreateModalVisible(false)}
                onSuccess={handleCreateSuccess}
            />

            {newApiKeyData && (
                <Card type="inner" title="新创建的通信密钥">
                    <Alert
                        message="请妥善保管此密钥，关闭后将无法再次查看完整密钥"
                        type="warning"
                        showIcon
                        className="mb-3"
                    />
                    <div className="space-y-2">
                        <div>
                            <span className="text-gray-600 dark:text-slate-400">密钥名称：</span>
                            <span className="font-medium">{newApiKeyData.name}</span>
                        </div>
                        <div>
                            <span className="text-gray-600 dark:text-slate-400">完整密钥：</span>
                            <code className="text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded font-mono break-all">
                                {newApiKeyData.key}
                            </code>
                        </div>
                    </div>
                    <Button
                        type="primary"
                        className="mt-3"
                        onClick={() => {
                            setShowApiKeyModal(false);
                            setNewApiKeyData(null);
                            if (newApiKeyData.id && onSelectApiKey) {
                                onSelectApiKey(newApiKeyData.id);
                            }
                        }}
                    >
                        使用此密钥
                    </Button>
                </Card>
            )}
        </>
    );
};

const getCommonCommands = (os: string) => {
    const agentCmd = os.startsWith('windows') ? `.\\${AGENT_NAME_EXE}` : AGENT_NAME;
    const sudo = os.startsWith('windows') ? '' : 'sudo ';

    return `# 查看服务状态
${sudo}${agentCmd} status

# 停止服务
${sudo}${agentCmd} stop

# 启动服务
${sudo}${agentCmd} start

# 重启服务
${sudo}${agentCmd} restart

# 卸载服务
${sudo}${agentCmd} uninstall

# 查看版本
${agentCmd} version`;
};

export const ServiceHelper = ({ os }: { os: string }) => (
    <Card title="服务管理命令">
        <Paragraph type="secondary" className="mb-3 text-gray-600 dark:text-slate-400">
            注册完成后，您可以使用以下命令管理探针服务：
        </Paragraph>
        <pre className="m-0 max-h-[320px] overflow-auto rounded-lg border border-slate-200 bg-slate-950 p-4 text-[13px] leading-6 text-slate-100 dark:border-slate-700">
            <code>{getCommonCommands(os)}</code>
        </pre>
    </Card>
);

export const ConfigHelper = () => (
    <Card title="配置文件说明">
        <Paragraph className="text-gray-900 dark:text-slate-100">
            注册完成后，配置文件会保存在:
        </Paragraph>
        <ul className="m-0 space-y-2 pl-5 text-gray-600 dark:text-slate-400">
            <li>
                <Text code>{CONFIG_PATH}</Text> - 配置文件路径
            </li>
            <li>
                您可以手动编辑此文件来修改配置，修改后需要重启服务生效
            </li>
        </ul>
    </Card>
);
