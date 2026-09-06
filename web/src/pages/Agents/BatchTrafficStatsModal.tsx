import {useEffect, useState} from 'react';
import {App, Form, InputNumber, Modal, Select, Space, Switch} from 'antd';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {updateTrafficConfig} from '@/api/agent';
import type {UpdateTrafficConfigRequest} from '@/types';
import {getErrorMessage} from '@/lib/utils';

type TrafficLimitUnit = 'GB' | 'TB';

interface BatchTrafficStatsModalProps {
    open: boolean;
    agentIds: string[];
    onCancel: () => void;
    onSuccess: () => void;
}

interface TrafficConfigFormValues {
    enabled: boolean;
    trafficType: UpdateTrafficConfigRequest['type'];
    trafficLimit: number;
    trafficResetDay: number;
}

const BYTES_PER_GB = 1024 * 1024 * 1024;
const BYTES_PER_TB = 1024 * BYTES_PER_GB;

const BatchTrafficStatsModal = ({open, agentIds, onCancel, onSuccess}: BatchTrafficStatsModalProps) => {
    const {message: messageApi} = App.useApp();
    const [form] = Form.useForm<TrafficConfigFormValues>();
    const queryClient = useQueryClient();
    const [limitUnit, setLimitUnit] = useState<TrafficLimitUnit>('GB');
    const enabled = Form.useWatch('enabled', form) ?? false;

    useEffect(() => {
        if (!open) {
            return;
        }
        form.setFieldsValue({
            enabled: false,
            trafficType: 'recv',
            trafficLimit: 0,
            trafficResetDay: 0,
        });
        setLimitUnit('GB');
    }, [open, form]);

    const handleUnitChange = (unit: TrafficLimitUnit) => {
        if (unit === limitUnit) {
            return;
        }

        const currentLimit = form.getFieldValue('trafficLimit') ?? 0;
        form.setFieldValue(
            'trafficLimit',
            unit === 'TB' ? currentLimit / 1024 : currentLimit * 1024,
        );
        setLimitUnit(unit);
    };

    const batchMutation = useMutation({
        mutationFn: async (payload: UpdateTrafficConfigRequest) => {
            const results = await Promise.allSettled(
                agentIds.map(agentId => updateTrafficConfig(agentId, payload)),
            );
            const failedCount = results.filter(result => result.status === 'rejected').length;
            return {
                total: agentIds.length,
                successCount: agentIds.length - failedCount,
                failedCount,
            };
        },
        onSuccess: (result) => {
            if (result.successCount === 0) {
                messageApi.error(`所选 ${result.total} 个探针的流量统计配置均失败，请重试`);
                return;
            }
            if (result.failedCount > 0) {
                messageApi.warning(`已配置 ${result.successCount} 个探针，失败 ${result.failedCount} 个`);
            } else {
                messageApi.success(`成功配置 ${result.total} 个探针的流量统计`);
            }
            queryClient.invalidateQueries({queryKey: ['admin', 'agents']});
            queryClient.invalidateQueries({queryKey: ['trafficStats']});
            onSuccess();
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '批量配置流量统计失败'));
        },
    });

    const handleOk = async () => {
        if (agentIds.length === 0) {
            messageApi.warning('请先选择要操作的探针');
            return;
        }

        try {
            const values = await form.validateFields();
            const bytesPerUnit = limitUnit === 'TB' ? BYTES_PER_TB : BYTES_PER_GB;
            await batchMutation.mutateAsync({
                enabled: values.enabled,
                type: values.trafficType || 'recv',
                limit: values.enabled ? Math.round((values.trafficLimit ?? 0) * bytesPerUnit) : 0,
                resetDay: values.enabled ? (values.trafficResetDay ?? 0) : 0,
            });
        } catch (error: unknown) {
            if (typeof error === 'object' && error !== null && 'errorFields' in error) {
                return;
            }
            messageApi.error(getErrorMessage(error, '批量配置流量统计失败'));
        }
    };

    return (
        <Modal
            title={`批量配置流量统计 (已选择 ${agentIds.length} 个探针)`}
            open={open}
            onOk={handleOk}
            onCancel={onCancel}
            confirmLoading={batchMutation.isPending}
            width={640}
            destroyOnHidden
        >
            <Form form={form} layout="vertical">
                <Form.Item
                    label="启用流量统计"
                    name="enabled"
                    valuePropName="checked"
                    extra="开启后将监控所选探针的流量使用情况"
                >
                    <Switch checkedChildren="已启用" unCheckedChildren="已禁用"/>
                </Form.Item>

                {enabled && (
                    <>
                        <Form.Item
                            label="统计类型"
                            name="trafficType"
                            rules={[{required: true, message: '请选择统计类型'}]}
                            extra="选择要统计的流量类型"
                        >
                            <Select
                                placeholder="请选择统计类型"
                                options={[
                                    {label: '进站流量 (下载)', value: 'recv'},
                                    {label: '出站流量 (上传)', value: 'send'},
                                    {label: '全部流量 (上传+下载)', value: 'both'},
                                ]}
                            />
                        </Form.Item>

                        <Form.Item
                            label="流量限额"
                            extra={`设置流量限额 (${limitUnit})，0 表示仅统计不告警`}
                        >
                            <Space.Compact style={{width: '100%'}}>
                                <Form.Item
                                    name="trafficLimit"
                                    noStyle
                                    rules={[{type: 'number', min: 0, message: '流量限额不能小于 0'}]}
                                >
                                    <InputNumber
                                        min={0}
                                        step={limitUnit === 'TB' ? 0.5 : 1}
                                        precision={limitUnit === 'TB' ? 2 : 0}
                                        placeholder="请输入流量限额"
                                        style={{width: '100%'}}
                                    />
                                </Form.Item>
                                <Select<TrafficLimitUnit>
                                    value={limitUnit}
                                    onChange={handleUnitChange}
                                    options={[
                                        {label: 'GB', value: 'GB'},
                                        {label: 'TB', value: 'TB'},
                                    ]}
                                    style={{width: 88}}
                                />
                            </Space.Compact>
                        </Form.Item>

                        <Form.Item
                            label="流量重置日期"
                            name="trafficResetDay"
                            rules={[{required: true, message: '请选择流量重置日期'}]}
                            extra="每月的几号重置流量，0 表示不自动重置"
                        >
                            <Select
                                placeholder="请选择流量重置日期"
                                options={[
                                    {label: '不自动重置', value: 0},
                                    ...Array.from({length: 31}, (_, index) => ({
                                        label: `每月 ${index + 1} 号`,
                                        value: index + 1,
                                    })),
                                ]}
                            />
                        </Form.Item>
                    </>
                )}
            </Form>
        </Modal>
    );
};

export default BatchTrafficStatsModal;
