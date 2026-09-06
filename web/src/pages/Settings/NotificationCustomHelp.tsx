import React from 'react';
import {Collapse, type CollapseProps} from "antd";

const NotificationCustomHelp = () => {

    const items: CollapseProps['items'] = [
        {
            key: 'help',
            label: '自定义请求体模板说明',
            children: <div className={'space-y-3 text-sm'}>
                {/* 自定义模板说明 */}
                <div className={'space-y-1'}>
                    <div className={'text-gray-600 dark:text-gray-400 text-xs'}>
                        支持变量替换，Content-Type 为 <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>application/json</code>
                    </div>
                    <div className={'border dark:border-gray-700 p-2 rounded-md text-xs mt-1 bg-gray-50 dark:bg-gray-800'}>
                        <div className={'font-semibold mb-1'}>可用变量：</div>
                        <div className={'grid grid-cols-2 gap-x-4 gap-y-1'}>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{message}}`}</code> - 告警消息</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{agent.id}}`}</code> - 探针ID</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{agent.name}}`}</code> - 探针名称</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{agent.hostname}}`}</code> - 主机名</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{agent.ip}}`}</code> - IP地址(合并)</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{agent.ipv4}}`}</code> - IPv4 地址</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{agent.ipv6}}`}</code> - IPv6 地址</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.type}}`}</code> - 告警类型</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.level}}`}</code> - 告警级别</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.status}}`}</code> - 告警状态</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.message}}`}</code> - 告警消息</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.threshold}}`}</code> - 阈值</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.actualValue}}`}</code> - 当前值</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.firedAt}}`}</code> - 触发时间(格式化)</div>
                            <div>• <code className={'bg-gray-100 dark:bg-gray-700 px-1 rounded'}>{`{{alert.resolvedAt}}`}</code> - 恢复时间(格式化)</div>
                        </div>
                        <div className={'mt-2 pt-2 border-t dark:border-gray-700'}>
                            <div className={'font-semibold mb-1'}>示例：</div>
                            <pre className={'text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded'}>
                                            {`{
  "alert": "{{alert.message}}",
  "host": "{{agent.hostname}}",
  "level": "{{alert.level}}"
}`}
                                        </pre>
                        </div>
                    </div>
                </div>
            </div>,
        },
    ];

    return <Collapse
        bordered={false}
        items={items}
    />;
};

export default NotificationCustomHelp;