import {useEffect, useState} from 'react';
import {App, Button, Form, Input, List, Modal, Space, Switch} from 'antd';
import {Plus, Shield, Trash2} from 'lucide-react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {updateTamperConfig} from '@/api/tamper';
import {getErrorMessage} from '@/lib/utils';

interface BatchTamperProtectionModalProps {
    open: boolean;
    agentIds: string[];
    onCancel: () => void;
    onSuccess: () => void;
}

const BatchTamperProtectionModal = ({open, agentIds, onCancel, onSuccess}: BatchTamperProtectionModalProps) => {
    const {message: messageApi} = App.useApp();
    const [form] = Form.useForm();
    const queryClient = useQueryClient();
    const [editPaths, setEditPaths] = useState<string[]>([]);
    const [newPath, setNewPath] = useState('');

    useEffect(() => {
        if (!open) {
            return;
        }
        form.setFieldsValue({enabled: false});
        setEditPaths([]);
        setNewPath('');
    }, [open, form]);

    const handleAddPath = () => {
        const trimmed = newPath.trim();
        if (trimmed && !editPaths.includes(trimmed)) {
            setEditPaths(prev => [...prev, trimmed]);
            setNewPath('');
        }
    };

    const handleRemovePath = (path: string) => {
        setEditPaths(prev => prev.filter(item => item !== path));
    };

    const batchMutation = useMutation({
        mutationFn: async (payload: {enabled: boolean; paths: string[]}) => {
            const results = await Promise.allSettled(
                agentIds.map(agentId => updateTamperConfig(agentId, payload.enabled, payload.paths)),
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
                messageApi.success(`成功配置 ${result.total} 个探针的防篡改保护`);
            }
            queryClient.invalidateQueries({queryKey: ['admin', 'agents']});
            queryClient.invalidateQueries({queryKey: ['tamperConfig']});
            onSuccess();
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '批量配置防篡改保护失败'));
        },
    });

    const handleOk = async () => {
        if (agentIds.length === 0) {
            messageApi.warning('请先选择要操作的探针');
            return;
        }
        try {
            const values = await form.validateFields();
            await batchMutation.mutateAsync({
                enabled: values.enabled,
                paths: editPaths,
            });
        } catch (error: unknown) {
            if (typeof error === 'object' && error !== null && 'errorFields' in error) {
                return;
            }
            messageApi.error(getErrorMessage(error, '批量配置防篡改保护失败'));
        }
    };

    return (
        <Modal
            title={`批量配置防篡改保护 (已选择 ${agentIds.length} 个探针)`}
            open={open}
            onOk={handleOk}
            onCancel={onCancel}
            confirmLoading={batchMutation.isPending}
            width={640}
            destroyOnHidden
        >
            <Space direction="vertical" size="large" style={{width: '100%'}}>
                <Form form={form} layout="vertical">
                    <Form.Item
                        label="启用防篡改保护"
                        name="enabled"
                        valuePropName="checked"
                        extra="开启后将对下方目录进行实时监控和保护"
                    >
                        <Switch checkedChildren="已启用" unCheckedChildren="已禁用"/>
                    </Form.Item>
                </Form>

                <div>
                    <div className="mb-2 text-sm font-medium">受保护的目录列表</div>
                    <Space.Compact style={{width: '100%'}} className="mb-3">
                        <Input
                            placeholder="输入要保护的目录路径，如 /etc/nginx"
                            value={newPath}
                            onChange={(e) => setNewPath(e.target.value)}
                            onPressEnter={handleAddPath}
                        />
                        <Button type="primary" icon={<Plus size={16}/>} onClick={handleAddPath}>
                            添加
                        </Button>
                    </Space.Compact>

                    {editPaths.length === 0 ? (
                        <div className="rounded border border-dashed border-gray-300 p-6 text-center">
                            <Shield size={40} className="mx-auto mb-2 text-gray-300"/>
                            <p className="text-sm text-gray-500">暂未配置保护目录</p>
                        </div>
                    ) : (
                        <List
                            bordered
                            dataSource={editPaths}
                            renderItem={(path) => (
                                <List.Item
                                    actions={[
                                        <Button
                                            key="delete"
                                            type="text"
                                            danger
                                            icon={<Trash2 size={16}/>}
                                            onClick={() => handleRemovePath(path)}
                                        />,
                                    ]}
                                >
                                    <span className="font-mono text-sm">{path}</span>
                                </List.Item>
                            )}
                        />
                    )}
                </div>
            </Space>
        </Modal>
    );
};

export default BatchTamperProtectionModal;
