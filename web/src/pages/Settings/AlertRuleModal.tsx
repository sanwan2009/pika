import {useEffect, useMemo} from 'react';
import {App, Form, Input, InputNumber, Modal, Radio, Select, Switch, TimePicker} from 'antd';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import dayjs from 'dayjs';
import {listAgentsByAdmin, getTags} from '@/api/agent.ts';
import {createAlertRule, updateAlertRule, type AlertRuleRequest} from '@/api/alertRule.ts';
import type {Agent, AlertRule} from '@/types';
import {getErrorMessage} from '@/lib/utils';
import {AlertRulesFields, DEFAULT_ALERT_RULES} from './AlertRulesFields';

// 新建规则时的默认事件通知开关（默认全部关闭，由用户按需开启）
const DEFAULT_ALERT_NOTIFICATIONS = {
    trafficEnabled: false,
    sshLoginSuccessEnabled: false,
    tamperEventEnabled: false,
    agentExpireEnabled: false,
};

interface AlertRuleModalProps {
    open: boolean;
    rule?: AlertRule;
    onCancel: () => void;
    onSuccess: () => void;
}

// 通知渠道类型（与通知渠道设置页的 7 种类型一致）
const CHANNEL_TYPE_OPTIONS = [
    {label: '钉钉', value: 'dingtalk'},
    {label: '企业微信', value: 'wecom'},
    {label: '企业微信应用', value: 'wecomApp'},
    {label: '飞书', value: 'feishu'},
    {label: 'Telegram', value: 'telegram'},
    {label: '邮件', value: 'email'},
    {label: '自定义 Webhook', value: 'webhook'},
];

const MAINTENANCE_TIME_PATTERN = /^(?:[01]\d|2[0-3]):[0-5]\d$/;

const createMaintenanceTimeRange = (startTime = '02:00', endTime = '02:20') => [
    dayjs(`2000-01-01T${MAINTENANCE_TIME_PATTERN.test(startTime) ? startTime : '02:00'}:00`),
    dayjs(`2000-01-01T${MAINTENANCE_TIME_PATTERN.test(endTime) ? endTime : '02:20'}:00`),
];

const AlertRuleModal = ({open, rule, onCancel, onSuccess}: AlertRuleModalProps) => {
    const [form] = Form.useForm();
    const {message: messageApi} = App.useApp();
    const queryClient = useQueryClient();
    const isEditMode = !!rule;

    const {data: agents = []} = useQuery({
        queryKey: ['agents', 'paging'],
        queryFn: async () => {
            const response = await listAgentsByAdmin();
            return response.data || [];
        },
        enabled: open,
    });

    const {data: tagsData} = useQuery({
        queryKey: ['agents', 'tags'],
        queryFn: getTags,
        enabled: open,
    });

    const agentOptions = useMemo(
        () =>
            agents.map((agent: Agent) => ({
                label: agent.name || agent.hostname || agent.id,
                value: agent.id,
            })),
        [agents],
    );

    const tagOptions = useMemo(
        () => (tagsData?.data?.tags || []).map((tag: string) => ({label: tag, value: tag})),
        [tagsData],
    );

    const watchTargetType = Form.useWatch('targetType', form) || 'all';
    const watchMaintenanceEnabled = Form.useWatch('maintenanceEnabled', form) ?? false;

    useEffect(() => {
        if (!open) {
            return;
        }
        form.resetFields();
        if (rule) {
            form.setFieldsValue({
                name: rule.name,
                priority: rule.priority ?? 0,
                enabled: rule.enabled,
                targetType: rule.targetType || 'all',
                agentIds: rule.agentIds || [],
                tags: rule.tags || [],
                channels: rule.channels || [],
                maskIP: rule.maskIP ?? false,
                notifications: {...DEFAULT_ALERT_NOTIFICATIONS, ...rule.notifications},
                maintenanceEnabled: rule.maintenanceEnabled ?? false,
                maintenanceTimeRange: createMaintenanceTimeRange(
                    rule.maintenanceStartTime,
                    rule.maintenanceEndTime,
                ),
                rules: rule.rules,
            });
        } else {
            form.setFieldsValue({
                name: '',
                priority: 10,
                enabled: true,
                targetType: 'all',
                agentIds: [],
                tags: [],
                channels: [],
                maskIP: false,
                notifications: DEFAULT_ALERT_NOTIFICATIONS,
                maintenanceEnabled: false,
                maintenanceTimeRange: createMaintenanceTimeRange(),
                rules: DEFAULT_ALERT_RULES,
            });
        }
    }, [open, rule, form]);

    const createMutation = useMutation({
        mutationFn: (data: AlertRuleRequest) => createAlertRule(data),
        onSuccess: () => {
            messageApi.success('规则创建成功');
            queryClient.invalidateQueries({queryKey: ['admin', 'alert-rules']});
            onSuccess();
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '创建失败'));
        },
    });

    const updateMutation = useMutation({
        mutationFn: (data: AlertRuleRequest) => updateAlertRule(rule!.id, data),
        onSuccess: () => {
            messageApi.success('规则保存成功');
            queryClient.invalidateQueries({queryKey: ['admin', 'alert-rules']});
            onSuccess();
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '保存失败'));
        },
    });

    const handleOk = async () => {
        try {
            const values = await form.validateFields();
            const [maintenanceStart, maintenanceEnd] = values.maintenanceTimeRange || createMaintenanceTimeRange();
            const payload: AlertRuleRequest = {
                name: values.name?.trim(),
                priority: values.priority ?? 0,
                enabled: values.enabled ?? true,
                targetType: values.targetType || 'all',
                agentIds: values.targetType === 'agents' ? (values.agentIds || []) : [],
                tags: values.targetType === 'tags' ? (values.tags || []) : [],
                channels: values.channels || [],
                maskIP: values.maskIP ?? false,
                notifications: values.notifications || DEFAULT_ALERT_NOTIFICATIONS,
                maintenanceEnabled: values.maintenanceEnabled ?? false,
                maintenanceStartTime: maintenanceStart.format('HH:mm'),
                maintenanceEndTime: maintenanceEnd.format('HH:mm'),
                rules: values.rules,
            };
            if (isEditMode) {
                await updateMutation.mutateAsync(payload);
            } else {
                await createMutation.mutateAsync(payload);
            }
        } catch (error: unknown) {
            if (typeof error === 'object' && error !== null && 'errorFields' in error) {
                return;
            }
            messageApi.error(getErrorMessage(error, isEditMode ? '保存失败' : '创建失败'));
        }
    };

    const isSubmitting = createMutation.isPending || updateMutation.isPending;

    return (
        <Modal
            title={isEditMode ? '编辑告警规则' : '新建告警规则'}
            open={open}
            onCancel={onCancel}
            onOk={handleOk}
            confirmLoading={isSubmitting}
            width={720}
            destroyOnHidden
        >
            <Form form={form} layout="vertical">
                <div className="grid gap-x-4 sm:grid-cols-2">
                    <Form.Item
                        label="规则名称"
                        name="name"
                        rules={[{required: true, message: '请输入规则名称'}]}
                    >
                        <Input placeholder="例如：生产环境主机"/>
                    </Form.Item>
                    <Form.Item
                        label="优先级"
                        name="priority"
                        rules={[{required: true, message: '请输入优先级'}]}
                        tooltip="数字越小越优先。主机同时命中多条规则时，仅优先级最高的一条生效。"
                    >
                        <InputNumber min={0} max={9999} style={{width: '100%'}}/>
                    </Form.Item>
                </div>

                <Form.Item label="适用主机" name="targetType" extra="未命中任何规则的主机不产生告警">
                    <Radio.Group
                        options={[
                            {label: '全部主机', value: 'all'},
                            {label: '按主机', value: 'agents'},
                            {label: '按标签', value: 'tags'},
                        ]}
                        optionType="button"
                        buttonStyle="solid"
                    />
                </Form.Item>

                {watchTargetType === 'agents' && (
                    <Form.Item
                        label="选择主机"
                        name="agentIds"
                        rules={[{required: true, message: '请选择主机'}]}
                    >
                        <Select
                            mode="multiple"
                            placeholder="选择主机（可多选）"
                            options={agentOptions}
                            allowClear
                            showSearch
                            optionFilterProp="label"
                        />
                    </Form.Item>
                )}

                {watchTargetType === 'tags' && (
                    <Form.Item
                        label="选择标签"
                        name="tags"
                        rules={[{required: true, message: '请选择标签'}]}
                        extra="拥有所选标签的主机都会命中此规则"
                    >
                        <Select
                            mode="multiple"
                            placeholder="选择标签（可多选）"
                            options={tagOptions}
                            allowClear
                        />
                    </Form.Item>
                )}

                <Form.Item
                    label="通知渠道"
                    name="channels"
                    extra="不选择则推送所有已启用的通知渠道"
                >
                    <Select
                        mode="multiple"
                        placeholder="推送所有已启用渠道"
                        options={CHANNEL_TYPE_OPTIONS}
                        allowClear
                    />
                </Form.Item>

                <Form.Item label="启用状态" name="enabled" valuePropName="checked">
                    <Switch checkedChildren="启用" unCheckedChildren="停用"/>
                </Form.Item>

                <Form.Item
                    label="IP 打码"
                    name="maskIP"
                    valuePropName="checked"
                    extra="开启后，此规则产生的通知消息中的 IP 地址将显示为 192.168.*.* 格式"
                >
                    <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
                </Form.Item>

                <div className="mb-2 text-[13px] font-semibold text-[#1f2329] dark:text-[#e6e8ec]">
                    计划维护
                </div>
                <Form.Item
                    label="每天定时免告警"
                    name="maintenanceEnabled"
                    valuePropName="checked"
                    extra="适用于定时重启、例行维护等场景。该时段内暂停本规则的监控告警，不生成告警记录，也不发送通知；时段结束后重新计算告警持续时间。事件通知和机器到期提醒不受影响。"
                >
                    <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
                </Form.Item>

                {watchMaintenanceEnabled && (
                    <Form.Item
                        label="每日免告警时间"
                        name="maintenanceTimeRange"
                        rules={[
                            {required: true, message: '请选择每日免告警时间'},
                            {
                                validator: async (_, value) => {
                                    if (value?.[0]?.format('HH:mm') === value?.[1]?.format('HH:mm')) {
                                        throw new Error('开始时间和结束时间不能相同');
                                    }
                                },
                            },
                        ]}
                        extra="按服务端时区每天生效，支持跨天，例如 23:50–00:20。"
                    >
                        <TimePicker.RangePicker
                            format="HH:mm"
                            minuteStep={5}
                            allowClear={false}
                            placeholder={['开始时间', '结束时间']}
                            style={{width: '100%'}}
                        />
                    </Form.Item>
                )}

                <div className="mb-2 text-[13px] font-semibold text-[#1f2329] dark:text-[#e6e8ec]">
                    事件通知
                </div>
                <div className="grid gap-x-4 sm:grid-cols-2 lg:grid-cols-4">
                    <Form.Item
                        label="流量告警通知"
                        name={['notifications', 'trafficEnabled']}
                        valuePropName="checked"
                    >
                        <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
                    </Form.Item>
                    <Form.Item
                        label="SSH 登录成功通知"
                        name={['notifications', 'sshLoginSuccessEnabled']}
                        valuePropName="checked"
                    >
                        <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
                    </Form.Item>
                    <Form.Item
                        label="防篡改事件通知"
                        name={['notifications', 'tamperEventEnabled']}
                        valuePropName="checked"
                    >
                        <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
                    </Form.Item>
                    <Form.Item
                        label="机器到期提醒"
                        name={['notifications', 'agentExpireEnabled']}
                        valuePropName="checked"
                        tooltip="配置到期时间后，提前 7、3、1 天各提醒一次"
                    >
                        <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
                    </Form.Item>
                </div>

                <div className="mb-2 text-[13px] font-semibold text-[#1f2329] dark:text-[#e6e8ec]">
                    告警规则
                </div>
                <AlertRulesFields/>
            </Form>
        </Modal>
    );
};

export default AlertRuleModal;
