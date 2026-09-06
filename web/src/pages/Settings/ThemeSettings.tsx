import {useEffect, useState} from 'react';
import {Alert, App, Button, Card, Empty, Popconfirm, Space, Spin, Tag, Typography, Upload} from 'antd';
import {Palette, ShieldAlert, Trash2, UploadCloud} from 'lucide-react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {activateTheme, deleteTheme, listThemes, type ThemeInfo, uploadTheme} from '@/api/theme';
import {getErrorMessage} from '@/lib/utils';

const {Text, Paragraph} = Typography;

const ThemePreview = ({theme}: {theme: ThemeInfo}) => {
    const [source, setSource] = useState<string>();

    useEffect(() => {
        const controller = new AbortController();
        let objectURL = '';
        const token = localStorage.getItem('token');

        fetch(theme.previewUrl, {
            signal: controller.signal,
            headers: token ? {Authorization: 'Bearer ' + token} : {},
        })
            .then((response) => response.ok ? response.blob() : Promise.reject())
            .then((blob) => {
                objectURL = URL.createObjectURL(blob);
                setSource(objectURL);
            })
            .catch(() => setSource(undefined));

        return () => {
            controller.abort();
            if (objectURL) URL.revokeObjectURL(objectURL);
        };
    }, [theme.id, theme.previewUrl]);

    return (
        <div className="flex aspect-[16/9] w-full items-center justify-center overflow-hidden bg-slate-100 dark:bg-slate-800">
            {source ? (
                <img
                    src={source}
                    alt={`${theme.name} 主题预览`}
                    className="block h-full w-full object-contain"
                />
            ) : (
                <div className="flex flex-col items-center gap-2 text-slate-400 dark:text-slate-500">
                    <Palette size={36}/>
                    <span className="text-sm">暂无预览图</span>
                </div>
            )}
        </div>
    );
};

const ThemeSettings = () => {
    const {message, modal} = App.useApp();
    const queryClient = useQueryClient();
    const themesQuery = useQuery({queryKey: ['themes'], queryFn: listThemes});
    const themes = themesQuery.data || [];
    const refresh = () => queryClient.invalidateQueries({queryKey: ['themes']});
    const operation = useMutation({
        mutationFn: async (fn: () => Promise<unknown>) => fn(),
        onSuccess: async () => {
            message.success('操作成功');
            await refresh();
        },
        onError: (error) => message.error(getErrorMessage(error, '主题操作失败')),
    });

    const confirmTrustedTheme = (title: string, action: () => Promise<unknown>) => {
        modal.confirm({
            title,
            icon: <ShieldAlert color="#faad14"/>,
            width: 560,
            content: (
                <div className="mt-4">
                    <Alert
                        type="warning"
                        showIcon
                        title="主题是同源可信代码"
                        description="主题包含可执行 JavaScript，能够访问此站点浏览器存储和公开 API。只安装并启用你完全信任的主题。"
                    />
                </div>
            ),
            okText: '我信任此主题，继续',
            okButtonProps: {danger: true},
            onOk: () => operation.mutateAsync(action),
        });
    };

    const renderThemeActions = (theme: ThemeInfo) => [
        theme.active ? (
            <Tag key="status" color="success">当前主题</Tag>
        ) : (
            <Button
                key="activate"
                type="link"
                disabled={!theme.compatible || operation.isPending}
                onClick={() => confirmTrustedTheme('启用 ' + theme.name, () => activateTheme(theme.id))}
            >
                启用
            </Button>
        ),
        theme.official ? (
            <Tag key="source">内置</Tag>
        ) : (
            <Popconfirm
                key="delete"
                title="确认删除主题？"
                description="主题文件删除后只能通过重新安装恢复。"
                disabled={theme.active || operation.isPending}
                onConfirm={() => operation.mutateAsync(() => deleteTheme(theme.id))}
            >
                <Button
                    type="link"
                    danger
                    disabled={theme.active || operation.isPending}
                    icon={<Trash2 size={15}/>}
                >
                    删除
                </Button>
            </Popconfirm>
        ),
    ];

    return (
        <div className="flex w-full min-w-0 flex-col gap-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                <div className="min-w-0 flex-1">
                    <Alert
                        type="warning"
                        showIcon
                        title="只安装你信任的主题：第三方主题会在 Pika 同一域名下执行 JavaScript，安装前请确认来源可信。"
                    />
                </div>
                <div className="shrink-0">
                    <Upload
                        accept=".zip,application/zip"
                        showUploadList={false}
                        disabled={operation.isPending}
                        beforeUpload={(file) => {
                            confirmTrustedTheme('上传并安装 ' + file.name, () => uploadTheme(file as File));
                            return false;
                        }}
                    >
                        <Button icon={<UploadCloud size={15}/>} loading={operation.isPending}>上传 ZIP</Button>
                    </Upload>
                </div>
            </div>

            {themesQuery.isLoading ? (
                <div className="flex min-h-72 items-center justify-center">
                    <Spin size="large" tip="正在加载主题...">
                        <div className="h-16 w-48"/>
                    </Spin>
                </div>
            ) : themesQuery.isError ? (
                <Alert
                    type="error"
                    showIcon
                    title="主题列表加载失败"
                    description={getErrorMessage(themesQuery.error, '请稍后重试')}
                    action={<Button size="small" onClick={() => themesQuery.refetch()}>重试</Button>}
                />
            ) : themes.length === 0 ? (
                <div className="rounded-lg border border-dashed border-slate-300 py-14 dark:border-slate-700">
                    <Empty description="暂无可用主题" image={Empty.PRESENTED_IMAGE_SIMPLE}/>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-3">
                    {themes.map((theme) => (
                        <div key={theme.id} className="min-w-0">
                            <Card
                                hoverable
                                cover={<ThemePreview theme={theme}/>}
                                actions={renderThemeActions(theme)}
                                styles={{
                                    root: {height: '100%', overflow: 'hidden'},
                                    body: {padding: 16},
                                    actions: {marginTop: 'auto'},
                                    cover: {overflow: 'hidden'},
                                }}
                            >
                                <Card.Meta
                                    title={(
                                        <Space size={[6, 6]} wrap>
                                            <Text strong>{theme.name}</Text>
                                            <Tag>{theme.version}</Tag>
                                            {theme.official && <Tag color="blue">官方</Tag>}
                                        </Space>
                                    )}
                                    description={(
                                        <Space orientation="vertical" size={10} style={{display: 'flex'}}>
                                            <Paragraph ellipsis={{rows: 2}} style={{margin: 0}}>
                                                {theme.description || '暂无说明'}
                                            </Paragraph>
                                            <Text type="secondary">作者：{theme.author}</Text>
                                            {!theme.compatible && (
                                                <Alert type="error" showIcon title="主题不兼容" description={theme.compatibilityError}/>
                                            )}
                                        </Space>
                                    )}
                                />
                            </Card>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};

export default ThemeSettings;
