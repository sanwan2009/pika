import {useEffect} from 'react';
import {App, Form, Input, Modal, Switch} from 'antd';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {updateSSHLoginConfig} from '@/api/agent';
import {getErrorMessage} from '@/lib/utils';

const {TextArea} = Input;

// 验证 IP 地址格式（支持 IPv4 和 CIDR）
const validateIPOrCIDR = (value: string): boolean => {
    const ipv4Regex = /^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
    const cidrRegex = /^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\/(3[0-2]|[12]?[0-9])$/;
    return ipv4Regex.test(value) || cidrRegex.test(value);
};

const parseIPWhitelist = (text: string): string[] => {
    return text
        .split('\n')
        .map(line => line.trim())
        .filter(line => line.length > 0);
};

interface BatchSSHLoginConfigModalProps {
    open: boolean;
    agentIds: string[];
    onCancel: () => void;
    onSuccess: () => void;
}

const BatchSSHLoginConfigModal = ({open, agentIds, onCancel, onSuccess}: BatchSSHLoginConfigModalProps) => {
    const {message: messageApi} = App.useApp();
    const [form] = Form.useForm();
    const queryClient = useQueryClient();

    useEffect(() => {
        if (!open) {
            return;
        }
        form.setFieldsValue({
            enabled: false,
            ipWhitelistText: '',
        });
    }, [open, form]);

    const batchMutation = useMutation({
        mutationFn: async (payload: {enabled: boolean; ipWhitelist: string[]}) => {
            const results = await Promise.allSettled(
                agentIds.map(agentId => updateSSHLoginConfig(agentId, payload)),
            );
            const failedCount = results.filter(result => result.status === 'rejected').length;
            return {
                total: agentIds.length,
                successCount: agentIds.length - failedCount,
                failedCount,
            };
        },
        onSuccess: (result) => {
            if (result.failedCount > 0) {
                messageApi.warning(`已配置 ${result.successCount} 个探针，失败 ${result.failedCount} 个`);
            } else {
                messageApi.success(`成功配置 ${result.total} 个探针的 SSH 登录监控`);
            }
            queryClient.invalidateQueries({queryKey: ['admin', 'agents']});
            queryClient.invalidateQueries({queryKey: ['sshLoginConfig']});
            onSuccess();
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '批量配置 SSH 登录监控失败'));
        },
    });

    const handleOk = async () => {
        if (agentIds.length === 0) {
            messageApi.warning('请先选择要操作的探针');
            return;
        }
        try {
            const values = await form.validateFields();
            const ipWhitelistText = values.ipWhitelistText || '';
            const ipWhitelist = parseIPWhitelist(ipWhitelistText);
            const invalidIPs = ipWhitelist.filter(ip => !validateIPOrCIDR(ip));
            if (invalidIPs.length > 0) {
                messageApi.error(`以下 IP 地址或 CIDR 格式不正确：\n${invalidIPs.join('\n')}`);
                return;
            }
            await batchMutation.mutateAsync({
                enabled: values.enabled,
                ipWhitelist: ipWhitelist,
            });
        } catch (error: unknown) {
            if (typeof error === 'object' && error !== null && 'errorFields' in error) {
                return;
            }
            messageApi.error(getErrorMessage(error, '批量配置 SSH 登录监控失败'));
        }
    };

    return (
        <Modal
            title={`批量配置 SSH 登录监控 (已选择 ${agentIds.length} 个探针)`}
            open={open}
            onOk={handleOk}
            onCancel={onCancel}
            confirmLoading={batchMutation.isPending}
            width={640}
            destroyOnHidden
        >
            <Form form={form} layout="vertical">
                <Form.Item
                    label="启用监控"
                    name="enabled"
                    valuePropName="checked"
                    extra="启用后，探针将自动安装 PAM Hook 并开始监控 SSH 登录事件"
                >
                    <Switch checkedChildren="已启用" unCheckedChildren="已禁用"/>
                </Form.Item>

                <Form.Item
                    label="IP 白名单"
                    name="ipWhitelistText"
                    extra="白名单中的 IP 地址登录时只记录不发送通知。每行一个 IP 地址或 CIDR 网段"
                >
                    <TextArea
                        rows={6}
                        placeholder={'每行一个 IP 地址或 CIDR 网段，例如：\n192.168.1.1\n192.168.1.0/24\n10.0.0.0/8'}
                    />
                </Form.Item>
            </Form>
        </Modal>
    );
};

export default BatchSSHLoginConfigModal;
