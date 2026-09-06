import {useEffect, useMemo} from 'react';
import {App, Button, Form, Input, InputNumber, Radio, Select, Spin, Switch} from 'antd';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import type {PublicIPConfig} from '@/api/property';
import {getPublicIPConfig, savePublicIPConfig} from '@/api/property';
import {listAgentsByAdmin, getTags} from '@/api/agent.ts';
import {getErrorMessage} from '@/lib/utils';
import type {Agent} from '@/types';
import {SettingsActions, SettingsSection} from './SettingsSection';

interface PublicIPConfigProps {
    defaultIPv4APIs: string[];
    defaultIPv6APIs: string[];
}

// 历史取值 custom 等同按探针
const normalizeScope = (scope: string | undefined): string =>
    scope === 'tags' ? 'tags' : scope === 'agents' || scope === 'custom' ? 'agents' : 'all';

const formatApiList = (apis: string[] | undefined, defaults: string[]) => {
    const list = apis && apis.length > 0 ? apis : defaults;
    return list.join('\n');
};

const parseApiList = (text: string, defaults: string[]) => {
    const items = text
        .split('\n')
        .map((item) => item.trim())
        .filter(Boolean);
    return items.length > 0 ? items : defaults;
};

const formatAgentLabel = (agent: Agent) => {
    if (agent.name && agent.hostname) {
        return `${agent.name} (${agent.hostname})`;
    }
    return agent.name || agent.hostname || agent.id;
};

const PublicIPConfigComponent = ({defaultIPv4APIs, defaultIPv6APIs}: PublicIPConfigProps) => {
    const [form] = Form.useForm();
    const {message: messageApi} = App.useApp();
    const queryClient = useQueryClient();

    const {data: config, isLoading} = useQuery({
        queryKey: ['publicIPConfig'],
        queryFn: getPublicIPConfig,
    });

    const {data: agentsResponse} = useQuery({
        queryKey: ['admin', 'agents', 'public-ip'],
        queryFn: () => listAgentsByAdmin(),
    });

    const {data: tagsData} = useQuery({
        queryKey: ['agents', 'tags'],
        queryFn: getTags,
    });

    const agentOptions = (agentsResponse?.data || []).map((agent) => ({
        label: formatAgentLabel(agent),
        value: agent.id,
    }));

    const tagOptions = useMemo(
        () => (tagsData?.data?.tags || []).map((tag: string) => ({label: tag, value: tag})),
        [tagsData],
    );

    const saveMutation = useMutation({
        mutationFn: savePublicIPConfig,
        onSuccess: () => {
            messageApi.success('保存成功');
            queryClient.invalidateQueries({queryKey: ['publicIPConfig']});
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '保存失败'));
        },
    });

    useEffect(() => {
        if (config) {
            form.setFieldsValue({
                enabled: config.enabled ?? false,
                intervalSeconds: config.intervalSeconds ?? 300,
                ipv4Scope: normalizeScope(config.ipv4Scope),
                ipv4AgentIds: config.ipv4AgentIds ?? [],
                ipv4Tags: config.ipv4Tags ?? [],
                ipv6Scope: normalizeScope(config.ipv6Scope),
                ipv6AgentIds: config.ipv6AgentIds ?? [],
                ipv6Tags: config.ipv6Tags ?? [],
                ipv4Enabled: config.ipv4Enabled ?? true,
                ipv6Enabled: config.ipv6Enabled ?? true,
                ipv4ApisText: formatApiList(config.ipv4Apis, defaultIPv4APIs),
                ipv6ApisText: formatApiList(config.ipv6Apis, defaultIPv6APIs),
            });
        }
    }, [config, form, defaultIPv4APIs, defaultIPv6APIs]);

    const handleSave = async () => {
        try {
            const values = await form.validateFields();
            const payload: PublicIPConfig = {
                enabled: values.enabled ?? false,
                intervalSeconds: values.intervalSeconds ?? 300,
                ipv4Scope: values.ipv4Scope ?? 'all',
                ipv4AgentIds: values.ipv4Scope === 'agents' ? values.ipv4AgentIds || [] : [],
                ipv4Tags: values.ipv4Scope === 'tags' ? values.ipv4Tags || [] : [],
                ipv6Scope: values.ipv6Scope ?? 'all',
                ipv6AgentIds: values.ipv6Scope === 'agents' ? values.ipv6AgentIds || [] : [],
                ipv6Tags: values.ipv6Scope === 'tags' ? values.ipv6Tags || [] : [],
                ipv4Enabled: values.ipv4Enabled ?? true,
                ipv6Enabled: values.ipv6Enabled ?? true,
                ipv4Apis: parseApiList(values.ipv4ApisText || '', defaultIPv4APIs),
                ipv6Apis: parseApiList(values.ipv6ApisText || '', defaultIPv6APIs),
            };
            saveMutation.mutate(payload);
        } catch (error) {
            // 表单验证失败
        }
    };

    const handleReset = () => {
        if (config) {
            form.setFieldsValue({
                enabled: config.enabled ?? false,
                intervalSeconds: config.intervalSeconds ?? 300,
                ipv4Scope: normalizeScope(config.ipv4Scope),
                ipv4AgentIds: config.ipv4AgentIds ?? [],
                ipv4Tags: config.ipv4Tags ?? [],
                ipv6Scope: normalizeScope(config.ipv6Scope),
                ipv6AgentIds: config.ipv6AgentIds ?? [],
                ipv6Tags: config.ipv6Tags ?? [],
                ipv4Enabled: config.ipv4Enabled ?? true,
                ipv6Enabled: config.ipv6Enabled ?? true,
                ipv4ApisText: formatApiList(config.ipv4Apis, defaultIPv4APIs),
                ipv6ApisText: formatApiList(config.ipv6Apis, defaultIPv6APIs),
            });
        }
    };

    const handleUseDefaults = () => {
        form.setFieldsValue({
            ipv4ApisText: defaultIPv4APIs.join('\n'),
            ipv6ApisText: defaultIPv6APIs.join('\n'),
        });
    };

    if (isLoading) {
        return (
            <div className="flex justify-center items-center py-20">
                <Spin/>
            </div>
        );
    }

    return (
        <div>
            <Form form={form} layout="vertical" onFinish={handleSave}>
                <SettingsSection title="采集设置" divided={false} description="通过探针定期获取公网出口 IP 地址。">
                    <div className="flex flex-wrap items-start gap-x-10 gap-y-4">
                        <Form.Item label="启用采集" name="enabled" valuePropName="checked">
                            <Switch/>
                        </Form.Item>
                        <Form.Item
                            label="采集间隔(秒)"
                            name="intervalSeconds"
                            rules={[{type: 'number', min: 30, message: '采集间隔不能小于 30 秒'}]}
                        >
                            <InputNumber min={30} max={86400}/>
                        </Form.Item>
                    </div>
                </SettingsSection>

                <SettingsSection title="IPv4 配置">
                    <Form.Item label="启用 IPv4" name="ipv4Enabled" valuePropName="checked">
                        <Switch/>
                    </Form.Item>
                    <Form.Item label="IPv4 采集范围" name="ipv4Scope">
                        <Radio.Group>
                            <Radio.Button value="all">全部探针</Radio.Button>
                            <Radio.Button value="agents">按探针</Radio.Button>
                            <Radio.Button value="tags">按标签</Radio.Button>
                        </Radio.Group>
                    </Form.Item>
                    <Form.Item noStyle shouldUpdate>
                        {({getFieldValue}) => {
                            const enabled = getFieldValue('ipv4Enabled');
                            const scope = getFieldValue('ipv4Scope');
                            if (!enabled || (scope !== 'agents' && scope !== 'tags')) {
                                return null;
                            }
                            if (scope === 'tags') {
                                return (
                                    <Form.Item
                                        label="选择标签"
                                        name="ipv4Tags"
                                        rules={[{required: true, message: '请选择至少一个标签'}]}
                                        extra="拥有所选标签的探针都会执行采集"
                                    >
                                        <Select
                                            mode="multiple"
                                            placeholder="选择标签（可多选）"
                                            options={tagOptions}
                                            allowClear
                                        />
                                    </Form.Item>
                                );
                            }
                            return (
                                <Form.Item
                                    label="选择探针"
                                    name="ipv4AgentIds"
                                    rules={[{required: true, message: '请选择至少一个探针'}]}
                                >
                                    <Select
                                        mode="multiple"
                                        placeholder="选择需要执行采集的探针"
                                        options={agentOptions}
                                        optionFilterProp="label"
                                        showSearch
                                    />
                                </Form.Item>
                            );
                        }}
                    </Form.Item>
                    <Form.Item
                        label="IPv4 API 列表"
                        name="ipv4ApisText"
                        tooltip="每行一个 HTTP/HTTPS API 地址"
                    >
                        <Input.TextArea rows={6} placeholder="每行一个 IPv4 API"/>
                    </Form.Item>
                </SettingsSection>

                <SettingsSection title="IPv6 配置">
                    <Form.Item label="启用 IPv6" name="ipv6Enabled" valuePropName="checked">
                        <Switch/>
                    </Form.Item>
                    <Form.Item label="IPv6 采集范围" name="ipv6Scope">
                        <Radio.Group>
                            <Radio.Button value="all">全部探针</Radio.Button>
                            <Radio.Button value="agents">按探针</Radio.Button>
                            <Radio.Button value="tags">按标签</Radio.Button>
                        </Radio.Group>
                    </Form.Item>
                    <Form.Item noStyle shouldUpdate>
                        {({getFieldValue}) => {
                            const enabled = getFieldValue('ipv6Enabled');
                            const scope = getFieldValue('ipv6Scope');
                            if (!enabled || (scope !== 'agents' && scope !== 'tags')) {
                                return null;
                            }
                            if (scope === 'tags') {
                                return (
                                    <Form.Item
                                        label="选择标签"
                                        name="ipv6Tags"
                                        rules={[{required: true, message: '请选择至少一个标签'}]}
                                        extra="拥有所选标签的探针都会执行采集"
                                    >
                                        <Select
                                            mode="multiple"
                                            placeholder="选择标签（可多选）"
                                            options={tagOptions}
                                            allowClear
                                        />
                                    </Form.Item>
                                );
                            }
                            return (
                                <Form.Item
                                    label="选择探针"
                                    name="ipv6AgentIds"
                                    rules={[{required: true, message: '请选择至少一个探针'}]}
                                >
                                    <Select
                                        mode="multiple"
                                        placeholder="选择需要执行采集的探针"
                                        options={agentOptions}
                                        optionFilterProp="label"
                                        showSearch
                                    />
                                </Form.Item>
                            );
                        }}
                    </Form.Item>
                    <Form.Item
                        label="IPv6 API 列表"
                        name="ipv6ApisText"
                        tooltip="每行一个 HTTP/HTTPS API 地址"
                    >
                        <Input.TextArea rows={6} placeholder="每行一个 IPv6 API"/>
                    </Form.Item>
                </SettingsSection>

                <SettingsActions>
                    <Button type="primary" htmlType="submit" loading={saveMutation.isPending}>
                        保存配置
                    </Button>
                    <Button onClick={handleReset}>
                        恢复当前配置
                    </Button>
                    <Button onClick={handleUseDefaults}>
                        使用默认 API
                    </Button>
                </SettingsActions>
            </Form>
        </div>
    );
};

export default PublicIPConfigComponent;
