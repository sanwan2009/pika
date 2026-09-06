import {useState} from 'react';
import {Alert, App, Button, Space, Spin} from 'antd';
import {RefreshCw, Shield} from 'lucide-react';
import {useQuery} from '@tanstack/react-query';
import {getAgentForAdmin, getAuditResult, sendAuditCommand, type VPSAuditResult} from '@/api/agent.ts';
import {getErrorMessage} from '@/lib/utils';
import AuditResultView from './AuditResultView';

interface AgentAuditProps {
    agentId: string;
}

const AgentAudit = ({agentId}: AgentAuditProps) => {
    const {message: messageApi} = App.useApp();
    const [auditing, setAuditing] = useState(false);

    // 获取探针信息
    const {data: agent} = useQuery({
        queryKey: ['admin', 'agent', agentId],
        queryFn: async () => {
            const response = await getAgentForAdmin(agentId);
            return response.data;
        },
        enabled: !!agentId,
    });

    // 获取审计结果
    const {
        data: auditResult,
        isLoading,
        refetch,
    } = useQuery({
        queryKey: ['admin', 'agent', agentId, 'audit'],
        queryFn: async () => {
            const response = await getAuditResult(agentId);
            return response.data;
        },
        enabled: !!agentId,
        retry: false,
    });

    const handleStartAudit = async () => {
        if (!agentId || !agent) return;

        // 检查是否为 Linux 系统
        if (!agent.os.toLowerCase().includes('linux')) {
            messageApi.warning('安全审计功能仅支持 Linux 系统');
            return;
        }

        setAuditing(true);
        try {
            await sendAuditCommand(agentId);
            messageApi.success('安全审计已启动，请稍后查看结果');

            // 10秒后刷新结果（给Server端分析时间）
            setTimeout(() => {
                refetch();
            }, 10000);
        } catch (error: unknown) {
            messageApi.error(getErrorMessage(error, '启动审计失败'));
        } finally {
            setAuditing(false);
        }
    };

    if (isLoading) {
        return (
            <div className="text-center py-12">
                <Spin/>
            </div>
        );
    }

    // 非 Linux 系统提示
    if (agent && !agent.os.toLowerCase().includes('linux')) {
        return (
            <Alert
                title="功能限制"
                description="安全审计功能仅支持 Linux 系统。当前系统为 Windows 或其他系统，无法使用此功能。"
                type="warning"
                showIcon
            />
        );
    }

    // 暂无审计结果
    if (!auditResult) {
        return (
            <Space orientation="vertical" style={{width: '100%'}} size="large">
                <Alert
                    title="暂无审计结果"
                    description={
                        <span>该探针还没有进行过安全审计。点击下方按钮来执行首次审计。</span>
                    }
                    type="info"
                    showIcon
                />
                <div className="flex justify-center">
                    <Button
                        type="primary"
                        size="large"
                        icon={<Shield size={16}/>}
                        onClick={handleStartAudit}
                        loading={auditing}
                    >
                        立即开始安全审计
                    </Button>
                </div>
            </Space>
        );
    }

    // 显示审计结果
    return (
        <Space orientation="vertical" style={{width: '100%'}} size="large">
            <div className="flex justify-end gap-2">
                <Button
                    icon={<RefreshCw size={16}/>}
                    onClick={() => refetch()}
                >
                    刷新结果
                </Button>
                <Button
                    type="primary"
                    icon={<Shield size={16}/>}
                    onClick={handleStartAudit}
                    loading={auditing}
                >
                    重新审计
                </Button>
            </div>
            <AuditResultView result={auditResult}/>
        </Space>
    );
};

export default AgentAudit;
