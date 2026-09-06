import {useState} from 'react';
import {App, Button, Switch, Tag, Tooltip} from 'antd';
import {Pencil, Plus, Trash2} from 'lucide-react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {listAlertRules, deleteAlertRule, enableAlertRule, disableAlertRule} from '@/api/alertRule.ts';
import type {AlertRule} from '@/types';
import {getErrorMessage} from '@/lib/utils';
import {SettingsSection} from './SettingsSection';
import AlertRuleModal from './AlertRuleModal';

// 通知渠道类型中文名（与通知渠道设置页保持一致）
const CHANNEL_TYPE_NAMES: Record<string, string> = {
    dingtalk: '钉钉',
    wecom: '企业微信',
    wecomApp: '企业微信应用',
    feishu: '飞书',
    telegram: 'Telegram',
    email: '邮件',
    webhook: '自定义 Webhook',
};

const TARGET_TYPE_NAMES: Record<string, string> = {
    all: '全部主机',
    agents: '按主机',
    tags: '按标签',
};

const AlertSettings = () => {
    const {message: messageApi, modal} = App.useApp();
    const queryClient = useQueryClient();
    const [ruleModalOpen, setRuleModalOpen] = useState(false);
    const [editingRule, setEditingRule] = useState<AlertRule | undefined>(undefined);

    // 告警规则列表
    const {data: rulePage} = useQuery({
        queryKey: ['admin', 'alert-rules'],
        queryFn: () => listAlertRules(),
    });

    const deleteRuleMutation = useMutation({
        mutationFn: (id: string) => deleteAlertRule(id),
        onSuccess: () => {
            messageApi.success('规则删除成功');
            queryClient.invalidateQueries({queryKey: ['admin', 'alert-rules']});
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '删除失败'));
        },
    });

    const toggleRuleMutation = useMutation({
        mutationFn: (rule: AlertRule) => (rule.enabled ? disableAlertRule(rule.id) : enableAlertRule(rule.id)),
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: ['admin', 'alert-rules']});
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '切换启用状态失败'));
        },
    });

    const handleDeleteRule = (rule: AlertRule) => {
        modal.confirm({
            title: '确认删除',
            content: `确定要删除告警规则「${rule.name}」吗？其中的主机将不再产生告警（除非命中其他规则）。`,
            okText: '删除',
            okType: 'danger',
            cancelText: '取消',
            onOk: async () => {
                await deleteRuleMutation.mutateAsync(rule.id);
            },
        });
    };

    const rules = rulePage?.data?.items || [];

    const renderRuleTarget = (rule: AlertRule) => {
        const typeName = TARGET_TYPE_NAMES[rule.targetType] || rule.targetType;
        if (rule.targetType === 'agents') {
            return (
                <Tooltip title={rule.agentNames?.join('、') || ''}>
                    <span>{typeName} · {rule.agentIds?.length || 0} 台</span>
                </Tooltip>
            );
        }
        if (rule.targetType === 'tags') {
            return (
                <span>
                    {typeName} · {(rule.tags || []).map((tag) => (
                        <Tag key={tag} color="green" style={{marginInlineEnd: 4}}>{tag}</Tag>
                    ))}
                </span>
            );
        }
        return <span>{typeName}</span>;
    };

    const renderRuleChannels = (rule: AlertRule) => {
        if (!rule.channels || rule.channels.length === 0) {
            return <Tag style={{margin: 0}}>全部渠道</Tag>;
        }
        return (
            <span className="flex flex-wrap items-center gap-1">
                {rule.channels.map((type) => (
                    <Tag key={type} color="blue" style={{margin: 0}}>
                        {CHANNEL_TYPE_NAMES[type] || type}
                    </Tag>
                ))}
            </span>
        );
    };

    return (
        <div>
            <SettingsSection
                title="告警规则"
                divided={false}
                description="主机按优先级（数字越小越优先）命中第一条启用的规则；未命中任何规则的主机不产生告警与通知。每条规则可配置阈值、每日计划维护、事件通知开关与推送渠道。"
                extra={(
                    <Button
                        type="primary"
                        icon={<Plus size={14}/>}
                        onClick={() => {
                            setEditingRule(undefined);
                            setRuleModalOpen(true);
                        }}
                    >
                        新建规则
                    </Button>
                )}
            >
                {rules.length === 0 ? (
                    <div className="rounded-xl border border-dashed border-[#e8ebf0] py-8 text-center text-sm text-[#98a0ab] dark:border-[#272b33] dark:text-[#7d8590]">
                        暂无告警规则，所有主机均不会产生告警
                    </div>
                ) : (
                    <div className="space-y-2">
                        {rules.map((rule) => (
                            <div
                                key={rule.id}
                                className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-[#e8ebf0] px-3.5 py-3 dark:border-[#272b33]"
                            >
                                <div className="min-w-0 flex-1">
                                    <div className="flex flex-wrap items-center gap-2">
                                        <span className="text-[13px] font-medium text-[#1f2329] dark:text-[#e6e8ec]">
                                            {rule.name}
                                        </span>
                                        <Tag color="gold" style={{margin: 0}}>
                                            优先级 {rule.priority}
                                        </Tag>
                                        {rule.maskIP && <Tag style={{margin: 0}}>IP打码</Tag>}
                                        {rule.maintenanceEnabled && (
                                            <Tag color="cyan" style={{margin: 0}}>
                                                每日免告警 {rule.maintenanceStartTime}–{rule.maintenanceEndTime}
                                            </Tag>
                                        )}
                                    </div>
                                    <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[#646a73] dark:text-[#9ba1ab]">
                                        {renderRuleTarget(rule)}
                                        {renderRuleChannels(rule)}
                                    </div>
                                </div>
                                <div className="flex shrink-0 items-center gap-2">
                                    <Switch
                                        size="small"
                                        checked={rule.enabled}
                                        loading={toggleRuleMutation.isPending && toggleRuleMutation.variables?.id === rule.id}
                                        onChange={() => toggleRuleMutation.mutate(rule)}
                                    />
                                    <Button
                                        type="text"
                                        size="small"
                                        icon={<Pencil size={14}/>}
                                        onClick={() => {
                                            setEditingRule(rule);
                                            setRuleModalOpen(true);
                                        }}
                                        title="编辑"
                                    />
                                    <Button
                                        type="text"
                                        size="small"
                                        danger
                                        icon={<Trash2 size={14}/>}
                                        onClick={() => handleDeleteRule(rule)}
                                        title="删除"
                                    />
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </SettingsSection>

            <AlertRuleModal
                open={ruleModalOpen}
                rule={editingRule}
                onCancel={() => setRuleModalOpen(false)}
                onSuccess={() => setRuleModalOpen(false)}
            />
        </div>
    );
};

export default AlertSettings;
