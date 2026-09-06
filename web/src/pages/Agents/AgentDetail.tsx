import {useEffect, useState} from 'react';
import {useNavigate, useParams, useSearchParams} from 'react-router-dom';
import type {TabsProps} from 'antd';
import {Alert, Spin, Tabs, Tag} from 'antd';
import {Activity, ArrowLeft, Edit, FileWarning, Lock, Shield, TrendingUp} from 'lucide-react';
import {useQuery, useQueryClient} from '@tanstack/react-query';
import {getAgentForAdmin, getTags} from '@/api/agent.ts';
import AgentBasicInfo from './AgentBasicInfo';
import AgentAudit from './AgentAudit';
import AgentEditModal from './AgentEditModal';
import TamperProtection from './TamperProtection';
import SSHLoginMonitor from './SSHLoginMonitor';
import TrafficStats from './TrafficStats';
import {PageHeader} from '@/components/PageHeader';
import {PagePanel} from '@/components/PagePanel';

const AgentDetail = () => {
    const {id} = useParams<{ id: string }>();
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [searchParams, setSearchParams] = useSearchParams();
    const [activeTab, setActiveTab] = useState<string>(searchParams.get('tab') || 'info');
    const [editModalVisible, setEditModalVisible] = useState(false);

    // 获取探针基本信息（用于显示头部卡片）
    const {data: agent, isLoading} = useQuery({
        queryKey: ['admin', 'agent', id],
        queryFn: async () => {
            if (!id) throw new Error('Agent ID is required');
            const response = await getAgentForAdmin(id);
            return response.data;
        },
        enabled: !!id,
    });

    // 获取标签列表（编辑探针弹窗使用）
    const {data: tags = []} = useQuery({
        queryKey: ['admin', 'agents', 'tags'],
        queryFn: async () => {
            const response = await getTags();
            return response.data.tags || [];
        },
    });

    useEffect(() => {
        // 同步 activeTab 到 URL
        const nextParams = new URLSearchParams(searchParams);
        if (nextParams.get('tab') === activeTab) {
            return;
        }
        nextParams.set('tab', activeTab);
        setSearchParams(nextParams);
    }, [activeTab, searchParams, setSearchParams]);

    if (isLoading) {
        return (
            <div className="text-center py-24">
                <Spin/>
            </div>
        );
    }

    if (!agent || !id) {
        return (
            <div className="text-center py-24">
                <Alert
                    title="未找到探针"
                    description="该探针不存在或已被删除"
                    type="error"
                    showIcon
                />
            </div>
        );
    }

    // Tab 项配置
    const tabItems: TabsProps['items'] = [
        {
            key: 'info',
            label: (
                <div className="flex items-center gap-2 text-sm">
                    <Activity size={16}/>
                    <div>基本信息</div>
                </div>
            ),
            children: <AgentBasicInfo agentId={id}/>,
        },
        {
            key: 'traffic',
            label: (
                <div className="flex items-center gap-2 text-sm">
                    <TrendingUp size={16}/>
                    <div>流量统计</div>
                </div>
            ),
            children: <TrafficStats agentId={id}/>,
        },
        {
            key: 'audit',
            label: (
                <div className="flex items-center gap-2 text-sm">
                    <Shield size={16}/>
                    <div>安全审计</div>
                </div>
            ),
            children: <AgentAudit agentId={id}/>,
        },
        {
            key: 'tamper',
            label: (
                <div className="flex items-center gap-2 text-sm">
                    <FileWarning size={16}/>
                    <div>防篡改保护</div>
                </div>
            ),
            children: agent.os.toLowerCase().includes('linux') ? (
                <TamperProtection agentId={id}/>
            ) : (
                <Alert
                    title="功能限制"
                    description="防篡改保护功能仅支持 Linux 系统。当前系统为 Windows 或其他系统，无法使用此功能。"
                    type="warning"
                    showIcon
                />
            ),
        },
        {
            key: 'ssh-login',
            label: (
                <div className="flex items-center gap-2 text-sm">
                    <Lock size={16}/>
                    <div>SSH 登录监控</div>
                </div>
            ),
            children: agent.os.toLowerCase().includes('linux') ? (
                <SSHLoginMonitor agentId={id}/>
            ) : (
                <Alert
                    title="功能限制"
                    description="SSH 登录监控功能仅支持 Linux 系统。当前系统为 Windows 或其他系统，无法使用此功能。"
                    type="warning"
                    showIcon
                />
            ),
        },
    ];

    return (
        <div className="space-y-4">
            <PageHeader
                title={
                    <span className="flex flex-wrap items-center gap-2.5">
                        {agent.name || agent.hostname}
                        <button
                            type="button"
                            onClick={() => navigate('/admin/agents')}
                            className="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-xs font-normal text-[#646a73] transition-colors hover:text-[#1677ff] dark:text-[#9ba1ab] dark:hover:text-[#75adff]"
                        >
                            <ArrowLeft size={12}/>
                            <span>返回列表</span>
                        </button>
                    </span>
                }
                extra={agent.status === 1 ? <Tag color="success">在线</Tag> : <Tag color="error">离线</Tag>}
                actions={[{
                    key: 'edit',
                    label: '编辑探针',
                    type: 'primary',
                    icon: <Edit size={16}/>,
                    onClick: () => setEditModalVisible(true),
                }]}
            />

            {/* Tabs 内容 */}
            <PagePanel>
                <Tabs
                    activeKey={activeTab}
                    onChange={setActiveTab}
                    items={tabItems}
                />
            </PagePanel>

            {/* 编辑探针模态框 */}
            <AgentEditModal
                open={editModalVisible}
                agentId={id}
                existingTags={tags}
                onCancel={() => setEditModalVisible(false)}
                onSuccess={() => {
                    setEditModalVisible(false);
                    queryClient.invalidateQueries({queryKey: ['admin', 'agent', id]});
                }}
            />
        </div>
    );
};

export default AgentDetail;
