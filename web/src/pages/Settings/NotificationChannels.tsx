import {useEffect, useState} from 'react';
import {App, Button, Form, Input, InputNumber, Select, Space, Spin, Switch, Tag} from 'antd';
import {Bell, Bot, Building2, ExternalLink, Mail, MessageSquare, Smartphone, TestTube, Webhook, type LucideIcon} from 'lucide-react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {
    getNotificationChannels,
    type NotificationChannel,
    saveNotificationChannels,
    testNotificationChannel,
} from '@/api/property.ts';
import {cn, getErrorMessage} from '@/lib/utils';
import NotificationCustomHelp from "@/pages/Settings/NotificationCustomHelp.tsx";

type ChannelType = 'dingtalk' | 'wecom' | 'wecomApp' | 'feishu' | 'telegram' | 'email' | 'webhook';

interface ChannelDefinition {
    type: ChannelType;
    name: string;
    description: string;
    enabledField: string;
    fieldPrefix: string;
    icon: LucideIcon;
    docsUrl?: string;
}

const CHANNELS: ChannelDefinition[] = [
    {
        type: 'dingtalk',
        name: '钉钉',
        description: '自定义机器人通知',
        enabledField: 'dingtalkEnabled',
        fieldPrefix: 'dingtalk',
        icon: Bell,
        docsUrl: 'https://open.dingtalk.com/document/robots/custom-robot-access',
    },
    {
        type: 'wecom',
        name: '企业微信',
        description: '群机器人通知',
        enabledField: 'wecomEnabled',
        fieldPrefix: 'wecomSecretKey',
        icon: Building2,
        docsUrl: 'https://work.weixin.qq.com/api/doc/90000/90136/91770',
    },
    {
        type: 'wecomApp',
        name: '企业微信应用',
        description: '应用消息通知',
        enabledField: 'wecomAppEnabled',
        fieldPrefix: 'wecomApp',
        icon: Smartphone,
        docsUrl: 'https://developer.work.weixin.qq.com/document/path/90236',
    },
    {
        type: 'feishu',
        name: '飞书',
        description: '自定义机器人通知',
        enabledField: 'feishuEnabled',
        fieldPrefix: 'feishu',
        icon: MessageSquare,
        docsUrl: 'https://www.feishu.cn/hc/zh-CN/articles/360024984973',
    },
    {
        type: 'telegram',
        name: 'Telegram',
        description: 'Bot 消息通知',
        enabledField: 'telegramEnabled',
        fieldPrefix: 'telegram',
        icon: Bot,
        docsUrl: 'https://core.telegram.org/bots/api',
    },
    {
        type: 'email',
        name: '邮件',
        description: 'SMTP 邮件通知',
        enabledField: 'emailEnabled',
        fieldPrefix: 'email',
        icon: Mail,
    },
    {
        type: 'webhook',
        name: '自定义 Webhook',
        description: '自定义 HTTP 请求',
        enabledField: 'webhookEnabled',
        fieldPrefix: 'webhook',
        icon: Webhook,
    },
];

const NotificationChannels = () => {
    const [form] = Form.useForm();
    const {message: messageApi} = App.useApp();
    const queryClient = useQueryClient();
    const [selectedType, setSelectedType] = useState<ChannelType>('dingtalk');
    const [dirty, setDirty] = useState(false);

    // 验证 token 字段，检查是否误输入了完整的 URL
    const validateToken = (_: any, value: string) => {
        if (value && (value.startsWith('http://') || value.startsWith('https://'))) {
            return Promise.reject(new Error('请只输入 Token，不要包含完整的 URL 地址'));
        }
        return Promise.resolve();
    };

    const validateTelegramProxyURL = (_: any, value?: string) => {
        if (!value) {
            return Promise.resolve();
        }

        try {
            const proxyURL = new URL(value);
            if (!['http:', 'socks5:'].includes(proxyURL.protocol) || !proxyURL.hostname) {
                throw new Error();
            }
            return Promise.resolve();
        } catch {
            return Promise.reject(new Error('请输入 http:// 或 socks5:// 开头的有效代理地址'));
        }
    };

    const validateTelegramAPIBaseURL = (_: any, value?: string) => {
        if (!value) {
            return Promise.resolve();
        }

        try {
            const apiBaseURL = new URL(value);
            if (
                !['http:', 'https:'].includes(apiBaseURL.protocol)
                || !apiBaseURL.hostname
                || apiBaseURL.search
                || apiBaseURL.hash
            ) {
                throw new Error();
            }
            return Promise.resolve();
        } catch {
            return Promise.reject(new Error('请输入 http:// 或 https:// 开头的有效反代地址'));
        }
    };

    // 监听各个通知渠道的启用状态
    const dingtalkEnabled = Form.useWatch('dingtalkEnabled', form);
    const wecomEnabled = Form.useWatch('wecomEnabled', form);
    const wecomAppEnabled = Form.useWatch('wecomAppEnabled', form);
    const feishuEnabled = Form.useWatch('feishuEnabled', form);
    const telegramEnabled = Form.useWatch('telegramEnabled', form);
    const emailEnabled = Form.useWatch('emailEnabled', form);
    const webhookEnabled = Form.useWatch('webhookEnabled', form);

    const enabledMap: Record<ChannelType, boolean | undefined> = {
        dingtalk: dingtalkEnabled,
        wecom: wecomEnabled,
        wecomApp: wecomAppEnabled,
        feishu: feishuEnabled,
        telegram: telegramEnabled,
        email: emailEnabled,
        webhook: webhookEnabled,
    };

    // 获取通知渠道列表
    const {data: channels = [], isLoading} = useQuery({
        queryKey: ['notificationChannels'],
        queryFn: getNotificationChannels,
    });

    // 保存 mutation
    const saveMutation = useMutation({
        mutationFn: saveNotificationChannels,
        onSuccess: () => {
            messageApi.success('保存成功');
            setDirty(false);
            queryClient.invalidateQueries({queryKey: ['notificationChannels']});
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '保存失败'));
        },
    });

    // 测试 mutation
    const testMutation = useMutation({
        mutationFn: testNotificationChannel,
        onSuccess: () => {
            messageApi.success('测试通知已发送');
        },
        onError: (error: unknown) => {
            messageApi.error(getErrorMessage(error, '测试失败'));
        },
    });

    // 将渠道数组转换为表单值
    useEffect(() => {
        if (channels.length > 0) {
            const formValues: Record<string, any> = {};

            channels.forEach((channel) => {
                if (channel.type === 'dingtalk') {
                    formValues.dingtalkEnabled = channel.enabled;
                    formValues.dingtalkSecretKey = channel.config?.secretKey || '';
                    formValues.dingtalkSignSecret = channel.config?.signSecret || '';
                } else if (channel.type === 'wecom') {
                    formValues.wecomEnabled = channel.enabled;
                    formValues.wecomSecretKey = channel.config?.secretKey || '';
                } else if (channel.type === 'wecomApp') {
                    formValues.wecomAppEnabled = channel.enabled;
                    formValues.wecomAppOrigin = channel.config?.origin || 'https://qyapi.weixin.qq.com';
                    formValues.wecomAppCorpId = channel.config?.corpId || '';
                    formValues.wecomAppCorpSecret = channel.config?.corpSecret || '';
                    formValues.wecomAppAgentId = channel.config?.agentId;
                    formValues.wecomAppToUser = channel.config?.toUser || '@all';
                } else if (channel.type === 'feishu') {
                    formValues.feishuEnabled = channel.enabled;
                    formValues.feishuSecretKey = channel.config?.secretKey || '';
                    formValues.feishuSignSecret = channel.config?.signSecret || '';
                } else if (channel.type === 'telegram') {
                    formValues.telegramEnabled = channel.enabled;
                    formValues.telegramBotToken = channel.config?.botToken || '';
                    formValues.telegramChatID = channel.config?.chatID || '';
                    formValues.telegramProxyURL = channel.config?.proxyURL || '';
                    formValues.telegramAPIBaseURL = channel.config?.apiBaseURL || '';
                } else if (channel.type === 'email') {
                    formValues.emailEnabled = channel.enabled;
                    formValues.emailSmtpHost = channel.config?.smtpHost || '';
                    formValues.emailSmtpPort = channel.config?.smtpPort || 587;
                    formValues.emailFromEmail = channel.config?.fromEmail || '';
                    formValues.emailPassword = channel.config?.password || '';
                    formValues.emailToEmail = channel.config?.toEmail || '';
                    formValues.emailSubject = channel.config?.subject || 'Pika 告警通知';
                } else if (channel.type === 'webhook') {
                    formValues.webhookEnabled = channel.enabled;
                    formValues.webhookUrl = channel.config?.url || '';
                    formValues.webhookMethod = channel.config?.method || 'POST';
                    formValues.webhookCustomBody = channel.config?.customBody || '';

                    // 解析 headers 为数组形式方便编辑
                    const headers = channel.config?.headers || {};
                    formValues.webhookHeaders = Object.entries(headers).map(([key, value]) => ({
                        key,
                        value
                    }));
                }
            });

            form.setFieldsValue(formValues);
        }
    }, [channels, form]);

    // 将表单值转换回渠道数组
    const handleSave = (values: any) => {
        const newChannels: NotificationChannel[] = [];

            // 钉钉
            if (values.dingtalkEnabled || values.dingtalkSecretKey) {
                newChannels.push({
                    type: 'dingtalk',
                    enabled: values.dingtalkEnabled || false,
                    config: {
                        secretKey: values.dingtalkSecretKey || '',
                        signSecret: values.dingtalkSignSecret || '',
                    },
                });
            }

            // 企业微信
            if (values.wecomEnabled || values.wecomSecretKey) {
                newChannels.push({
                    type: 'wecom',
                    enabled: values.wecomEnabled || false,
                    config: {
                        secretKey: values.wecomSecretKey || '',
                    },
                });
            }

            // 企业微信应用
            if (values.wecomAppEnabled || values.wecomAppOrigin) {
                newChannels.push({
                    type: 'wecomApp',
                    enabled: values.wecomAppEnabled || false,
                    config: {
                        origin: values.wecomAppOrigin || '',
                        corpId: values.wecomAppCorpId || '',
                        corpSecret: values.wecomAppCorpSecret || '',
                        agentId: values.wecomAppAgentId,
                        toUser: values.wecomAppToUser || '',
                    },
                });
            }

            // 飞书
            if (values.feishuEnabled || values.feishuSecretKey) {
                newChannels.push({
                    type: 'feishu',
                    enabled: values.feishuEnabled || false,
                    config: {
                        secretKey: values.feishuSecretKey || '',
                        signSecret: values.feishuSignSecret || '',
                    },
                });
            }

            // Telegram
            if (values.telegramEnabled || values.telegramBotToken) {
                newChannels.push({
                    type: 'telegram',
                    enabled: values.telegramEnabled || false,
                    config: {
                        botToken: values.telegramBotToken || '',
                        chatID: values.telegramChatID || '',
                        proxyURL: values.telegramProxyURL || '',
                        apiBaseURL: values.telegramAPIBaseURL || '',
                    },
                });
            }

            // 邮件
            if (values.emailEnabled || values.emailSmtpHost) {
                newChannels.push({
                    type: 'email',
                    enabled: values.emailEnabled || false,
                    config: {
                        smtpHost: values.emailSmtpHost || '',
                        smtpPort: values.emailSmtpPort || 587,
                        fromEmail: values.emailFromEmail || '',
                        password: values.emailPassword || '',
                        toEmail: values.emailToEmail || '',
                        subject: values.emailSubject || 'Pika 告警通知',
                    },
                });
            }

            // 自定义Webhook
            if (values.webhookEnabled || values.webhookUrl) {
                // 将 headers 数组转换为对象
                const headersObj: Record<string, string> = {};
                if (values.webhookHeaders && Array.isArray(values.webhookHeaders)) {
                    values.webhookHeaders.forEach((item: { key: string; value: string }) => {
                        if (item.key && item.value) {
                            headersObj[item.key] = item.value;
                        }
                    });
                }

                newChannels.push({
                    type: 'webhook',
                    enabled: values.webhookEnabled || false,
                    config: {
                        url: values.webhookUrl || '',
                        method: values.webhookMethod || 'POST',
                        customBody: values.webhookCustomBody || '',
                        headers: Object.keys(headersObj).length > 0 ? headersObj : undefined,
                    },
                });
            }

            saveMutation.mutate(newChannels);
    };

    // 校验失败时跳转到第一个出错的渠道（错误可能位于未选中的渠道分组中）
    const handleValidateFailed = (errorFields: { name: (string | number)[] }[]) => {
        const firstField = errorFields?.[0]?.name?.[0];
        if (typeof firstField === 'string') {
            const matched = [...CHANNELS]
                .sort((a, b) => b.fieldPrefix.length - a.fieldPrefix.length)
                .find((channel) => firstField.startsWith(channel.fieldPrefix));
            if (matched) {
                setSelectedType(matched.type);
            }
        }
        messageApi.warning('存在未填写的必填项，请检查对应渠道的配置');
    };

    const selectedChannel = CHANNELS.find((channel) => channel.type === selectedType) ?? CHANNELS[0];
    const selectedEnabled = enabledMap[selectedChannel.type];
    const testingSelected = testMutation.isPending && testMutation.variables === selectedChannel.type;

    if (isLoading) {
        return (
            <div className="flex justify-center items-center py-20">
                <Spin/>
            </div>
        );
    }

    const fieldGridClass = 'grid gap-x-4 sm:grid-cols-2';

    const renderChannelFields = (channel: ChannelDefinition) => {
        switch (channel.type) {
            case 'dingtalk':
                return (
                    <div className={fieldGridClass}>
                        <Form.Item
                            label="访问令牌 (Access Token)"
                            name="dingtalkSecretKey"
                            rules={[
                                {required: true, message: '请输入访问令牌'},
                                {validator: validateToken}
                            ]}
                            tooltip="在钉钉机器人配置中获取的 access_token"
                        >
                            <Input placeholder="输入访问令牌"/>
                        </Form.Item>
                        <Form.Item
                            label="加签密钥（可选）"
                            name="dingtalkSignSecret"
                            tooltip="如果启用了加签，请填写 SEC 开头的密钥"
                        >
                            <Input.Password placeholder="SEC 开头的加签密钥"/>
                        </Form.Item>
                    </div>
                );
            case 'wecom':
                return (
                    <Form.Item
                        label="Webhook Key"
                        name="wecomSecretKey"
                        rules={[
                            {required: true, message: '请输入 Webhook Key'},
                            {validator: validateToken}
                        ]}
                        tooltip="企业微信群机器人的 Webhook Key"
                    >
                        <Input placeholder="输入 Webhook Key"/>
                    </Form.Item>
                );
            case 'wecomApp':
                return (
                    <div className="space-y-0">
                        <Form.Item
                            label="origin"
                            name="wecomAppOrigin"
                            initialValue="https://qyapi.weixin.qq.com"
                            rules={[{required: true, message: '请输入企业微信应用origin'}]}
                            tooltip="企业微信应用origin， Pika部署在可信IP的服务器下保持默认即可"
                        >
                            <Input placeholder="https://qyapi.weixin.qq.com"/>
                        </Form.Item>
                        <div className={fieldGridClass}>
                            <Form.Item
                                label="corpid"
                                name="wecomAppCorpId"
                                rules={[{required: true, message: '请输入企业微信的corpid'}]}
                                tooltip="企业微信的corpid"
                            >
                                <Input placeholder="输入您的企业的corpid"/>
                            </Form.Item>
                            <Form.Item
                                label="agentid"
                                name="wecomAppAgentId"
                                rules={[{required: true, message: '请输入企业微信应用的agentid'}]}
                                tooltip="企业微信应用的agentid"
                            >
                                <InputNumber style={{width: '100%'}}
                                             placeholder="输入您的企业应用的agentid"/>
                            </Form.Item>
                            <Form.Item
                                label="corpsecret"
                                name="wecomAppCorpSecret"
                                rules={[{required: true, message: '请输入企业微信应用的corpsecret'}]}
                                tooltip="企业微信应用的corpsecret"
                            >
                                <Input.Password placeholder="输入您的企业应用的corpsecret"/>
                            </Form.Item>
                            <Form.Item
                                label="touser"
                                name="wecomAppToUser"
                                initialValue="@all"
                                rules={[{required: true, message: '请输入接收消息的用户'}]}
                                tooltip="接收告警消息的用户"
                            >
                                <Input placeholder="输入接收告警消息的用户，全部可填@all"/>
                            </Form.Item>
                        </div>
                    </div>
                );
            case 'feishu':
                return (
                    <div className={fieldGridClass}>
                        <Form.Item
                            label="Webhook Token"
                            name="feishuSecretKey"
                            rules={[
                                {required: true, message: '请输入 Webhook Token'},
                                {validator: validateToken}
                            ]}
                            tooltip="飞书群机器人的 Webhook Token"
                        >
                            <Input placeholder="输入 Webhook Token"/>
                        </Form.Item>
                        <Form.Item
                            label="签名密钥（可选）"
                            name="feishuSignSecret"
                            tooltip="如果启用了签名验证，请填写密钥"
                        >
                            <Input.Password placeholder="输入签名密钥"/>
                        </Form.Item>
                    </div>
                );
            case 'telegram':
                return (
                    <div className="space-y-0">
                        <div className={fieldGridClass}>
                            <Form.Item
                                label="Bot Token"
                                name="telegramBotToken"
                                rules={[
                                    {required: true, message: '请输入 Bot Token'},
                                    {validator: validateToken}
                                ]}
                                tooltip="通过 @BotFather 创建机器人后获得的 token"
                            >
                                <Input.Password placeholder="输入 Bot Token"/>
                            </Form.Item>
                            <Form.Item
                                label="Chat ID"
                                name="telegramChatID"
                                rules={[{required: true, message: '请输入 Chat ID'}]}
                                tooltip="可以是用户 ID、群组 ID 或频道 ID，通过 @userinfobot 等机器人获取"
                            >
                                <Input placeholder="输入 Chat ID，例如：123456789"/>
                            </Form.Item>
                            <Form.Item
                                label="代理地址（HTTP/SOCKS5，可选）"
                                name="telegramProxyURL"
                                rules={[{validator: validateTelegramProxyURL}]}
                                tooltip="支持 HTTP 和 SOCKS5 代理；如需认证，请将账号密码写在代理 URL 中"
                            >
                                <Input.Password
                                    autoComplete="new-password"
                                    placeholder="例如：http://user:pass@127.0.0.1:7890"
                                />
                            </Form.Item>
                            <Form.Item
                                label="自定义反代地址（可选）"
                                name="telegramAPIBaseURL"
                                rules={[{validator: validateTelegramAPIBaseURL}]}
                                tooltip="Telegram Bot API 的反向代理基址；请求路径会自动拼接 /bot{token}/sendMessage"
                            >
                                <Input placeholder="例如：https://telegram.example.com"/>
                            </Form.Item>
                        </div>
                    </div>
                );
            case 'email':
                return (
                    <div className="space-y-0">
                        <div className={fieldGridClass}>
                            <Form.Item
                                label="SMTP 服务器"
                                name="emailSmtpHost"
                                rules={[{required: true, message: '请输入 SMTP 服务器地址'}]}
                                tooltip="邮件服务商的 SMTP 服务器地址，如 smtp.gmail.com"
                            >
                                <Input placeholder="例如：smtp.gmail.com"/>
                            </Form.Item>
                            <Form.Item
                                label="SMTP 端口"
                                name="emailSmtpPort"
                                rules={[{required: true, message: '请输入 SMTP 端口'}]}
                                tooltip="通常为 587（STARTTLS）或 465（SSL/TLS）"
                            >
                                <Input type="number" placeholder="587"/>
                            </Form.Item>
                            <Form.Item
                                label="发件人邮箱"
                                name="emailFromEmail"
                                rules={[
                                    {required: true, message: '请输入发件人邮箱'},
                                    {type: 'email', message: '请输入有效的邮箱地址'}
                                ]}
                                tooltip="用于发送告警邮件的邮箱地址"
                            >
                                <Input placeholder="your-email@example.com"/>
                            </Form.Item>
                            <Form.Item
                                label="收件人邮箱"
                                name="emailToEmail"
                                rules={[
                                    {required: true, message: '请输入收件人邮箱'},
                                    {type: 'email', message: '请输入有效的邮箱地址'}
                                ]}
                                tooltip="接收告警邮件的邮箱地址"
                            >
                                <Input placeholder="receiver@example.com"/>
                            </Form.Item>
                            <Form.Item
                                label="邮箱密码/授权码"
                                name="emailPassword"
                                rules={[{required: true, message: '请输入邮箱密码或授权码'}]}
                                tooltip="某些邮件服务商（如 Gmail、QQ 邮箱）需要使用授权码而非密码"
                            >
                                <Input.Password placeholder="输入邮箱密码或授权码"/>
                            </Form.Item>
                            <Form.Item
                                label="邮件主题"
                                name="emailSubject"
                                tooltip="告警邮件的主题，默认为 'Pika 告警通知'"
                            >
                                <Input placeholder="Pika 告警通知"/>
                            </Form.Item>
                        </div>
                    </div>
                );
            case 'webhook':
                return (
                    <div className="space-y-0">
                        <Form.Item
                            label="Webhook URL"
                            name="webhookUrl"
                            rules={[
                                {required: true, message: '请输入自定义 Webhook URL'},
                                {type: 'url', message: '请输入有效的 URL'},
                            ]}
                        >
                            <Input placeholder="https://your-server.com/webhook"/>
                        </Form.Item>
                        {/* HTTP 方法 */}
                        <Form.Item
                            label="HTTP 方法"
                            name="webhookMethod"
                            tooltip="选择 HTTP 请求方法"
                        >
                            <Select
                                placeholder="选择 HTTP 方法"
                                options={[
                                    {label: 'GET', value: 'GET'},
                                    {label: 'POST', value: 'POST'},
                                    {label: 'PUT', value: 'PUT'},
                                    {label: 'PATCH', value: 'PATCH'},
                                    {label: 'DELETE', value: 'DELETE'},
                                ]}
                            />
                        </Form.Item>

                        {/* 自定义请求体 */}
                        <Form.Item
                            label="自定义请求体"
                            name="webhookCustomBody"
                            rules={[
                                {
                                    required: true,
                                    message: '请输入自定义请求体模板'
                                }
                            ]}
                            tooltip="支持变量替换，可用变量见下方说明"
                        >
                            <Input.TextArea
                                rows={6}
                                placeholder='示例: {"alert": "{{alert.message}}", "host": "{{agent.hostname}}"}'
                            />
                        </Form.Item>

                        {/* 自定义请求头 */}
                        <Form.Item label="自定义请求头"
                                   tooltip="添加自定义 HTTP 请求头">
                            <Form.List name="webhookHeaders">
                                {(fields, {add, remove}) => (
                                    <>
                                        {fields.map(({
                                                         key,
                                                         name,
                                                         ...restField
                                                     }) => (
                                            <Space
                                                key={key}
                                                style={{
                                                    display: 'flex',
                                                    marginBottom: 8
                                                }}
                                                align="baseline"
                                            >
                                                <Form.Item
                                                    {...restField}
                                                    name={[name, 'key']}
                                                    rules={[{
                                                        required: true,
                                                        message: '请输入 Header 名称'
                                                    }]}
                                                >
                                                    <Input
                                                        placeholder="Header 名称"
                                                        style={{width: 200}}
                                                    />
                                                </Form.Item>
                                                <Form.Item
                                                    {...restField}
                                                    name={[name, 'value']}
                                                    rules={[{
                                                        required: true,
                                                        message: '请输入 Header 值'
                                                    }]}
                                                >
                                                    <Input
                                                        placeholder="Header 值"
                                                        style={{width: 300}}
                                                    />
                                                </Form.Item>
                                                <Button
                                                    onClick={() => remove(name)}
                                                    danger
                                                    type="link"
                                                >
                                                    删除
                                                </Button>
                                            </Space>
                                        ))}
                                        <Form.Item>
                                            <Button
                                                type="dashed"
                                                onClick={() => add()}
                                                block
                                            >
                                                添加请求头
                                            </Button>
                                        </Form.Item>
                                    </>
                                )}
                            </Form.List>
                        </Form.Item>
                    </div>
                );
        }
    };

    const SelectedIcon = selectedChannel.icon;

    return (
        <Form
            form={form}
            layout="vertical"
            onFinish={handleSave}
            onFinishFailed={({errorFields}) => handleValidateFailed(errorFields)}
            onValuesChange={() => setDirty(true)}
        >
            <div className="grid items-start gap-6 xl:grid-cols-[248px_minmax(0,1fr)]">
                {/* 渠道列表 */}
                <div>
                    <div className="mb-2 px-2 text-xs text-[#646a73] dark:text-[#9ba1ab]">
                        点击渠道查看配置，开关控制启用状态
                    </div>
                    <div className="grid gap-1.5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-1">
                        {CHANNELS.map((channel) => {
                            const Icon = channel.icon;
                            const enabled = enabledMap[channel.type];
                            const selected = channel.type === selectedType;
                            return (
                                <div
                                    key={channel.type}
                                    className={cn(
                                        'flex min-w-0 items-center gap-2 rounded-[10px] px-2 py-1.5 transition-colors',
                                        selected
                                            ? 'bg-[#eaf2ff] dark:bg-[#1677ff]/15'
                                            : 'hover:bg-[#f5f6f8] dark:hover:bg-[#20242c]',
                                    )}
                                >
                                    <button
                                        type="button"
                                        onClick={() => setSelectedType(channel.type)}
                                        className="flex min-w-0 flex-1 cursor-pointer items-center gap-2.5 border-0 bg-transparent p-0 text-left"
                                        aria-current={selected ? 'true' : undefined}
                                    >
                                        <span className={cn(
                                            'grid h-9 w-9 shrink-0 place-items-center rounded-lg transition-colors',
                                            selected
                                                ? 'bg-white text-[#145dcc] dark:bg-[#2a3038] dark:text-[#75adff]'
                                                : 'bg-[#f0f2f5] text-[#646a73] dark:bg-[#20242c] dark:text-[#9ba1ab]',
                                        )}>
                                            <Icon size={16}/>
                                        </span>
                                        <span className="min-w-0 flex-1">
                                            <span className="block truncate text-[13px] font-medium text-[#1f2329] dark:text-[#e6e8ec]">
                                                {channel.name}
                                            </span>
                                            <span className="block truncate text-xs text-[#98a0ab] dark:text-[#7d8590]">
                                                {enabled ? '已启用' : '未启用'}
                                            </span>
                                        </span>
                                    </button>
                                    <Form.Item noStyle name={channel.enabledField} valuePropName="checked">
                                        <Switch size="small" aria-label={`启用${channel.name}`}/>
                                    </Form.Item>
                                </div>
                            );
                        })}
                    </div>
                </div>

                {/* 选中渠道的配置 */}
                <div className="min-w-0">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                        <div className="flex items-center gap-3">
                            <span className="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400">
                                <SelectedIcon size={20}/>
                            </span>
                            <div>
                                <div className="flex items-center gap-2">
                                    <span className="text-[15px] font-semibold text-[#1f2329] dark:text-[#e6e8ec]">
                                        {selectedChannel.name}
                                    </span>
                                    <Tag color={selectedEnabled ? 'success' : 'default'} style={{margin: 0}}>
                                        {selectedEnabled ? '已启用' : '未启用'}
                                    </Tag>
                                </div>
                                <div className="mt-0.5 text-xs text-[#646a73] dark:text-[#9ba1ab]">
                                    {selectedChannel.description}
                                </div>
                            </div>
                        </div>
                        <div className="flex shrink-0 items-center gap-2">
                            {selectedChannel.docsUrl && (
                                <Button
                                    icon={<ExternalLink size={14}/>}
                                    href={selectedChannel.docsUrl}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    接入文档
                                </Button>
                            )}
                            <Button
                                icon={<TestTube size={14}/>}
                                onClick={() => testMutation.mutate(selectedChannel.type)}
                                loading={testingSelected}
                                disabled={!selectedEnabled || dirty}
                                title={dirty ? '配置已修改，请先保存再测试' : undefined}
                            >
                                发送测试
                            </Button>
                        </div>
                    </div>

                    <div className="my-4 border-t border-[#e8ebf0] dark:border-[#272b33]"/>

                    {!selectedEnabled && (
                        <div className="flex items-center gap-2 rounded-lg bg-[#f5f6f8] px-3.5 py-2.5 text-xs text-[#646a73] dark:bg-[#1c2028] dark:text-[#9ba1ab]">
                            <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-current opacity-60"/>
                            当前渠道未启用，打开左侧的开关后即可编辑配置。
                        </div>
                    )}

                    {/* 已启用渠道的表单项保持挂载（仅隐藏未选中的），保证校验行为一致 */}
                    {CHANNELS.filter((channel) => enabledMap[channel.type]).map((channel) => (
                        <div key={channel.type} className={channel.type === selectedType ? undefined : 'hidden'}>
                            {renderChannelFields(channel)}
                        </div>
                    ))}

                    {selectedType === 'webhook' && <NotificationCustomHelp/>}

                    <div className="mt-5 flex flex-wrap items-center gap-3 border-t border-[#e8ebf0] pt-4 dark:border-[#272b33]">
                        <Button type="primary" htmlType="submit" loading={saveMutation.isPending}>
                            保存配置
                        </Button>
                        {dirty && (
                            <span className="text-xs text-amber-600 dark:text-amber-400">
                                配置尚未保存，保存后才能发送测试通知。
                            </span>
                        )}
                    </div>
                </div>
            </div>
        </Form>
    );
};

export default NotificationChannels;
