import {Form, InputNumber, Switch} from 'antd';

export interface RuleDefinition {
    key: string;
    name: string;
    threshold?: {
        label: string;
        min: number;
        max: number;
        tooltip?: string;
    };
    duration?: {
        tooltip?: string;
    };
}

export const ALERT_RULES: RuleDefinition[] = [
    {key: 'cpu', name: 'CPU 使用率', threshold: {label: '阈值 (%)', min: 0, max: 100}, duration: {}},
    {key: 'memory', name: '内存使用率', threshold: {label: '阈值 (%)', min: 0, max: 100}, duration: {}},
    {key: 'disk', name: '磁盘使用率', threshold: {label: '阈值 (%)', min: 0, max: 100}, duration: {}},
    {key: 'network', name: '网速', threshold: {label: '阈值 (MB/s)', min: 0, max: 10000}, duration: {}},
    {
        key: 'cert',
        name: 'HTTPS 证书',
        threshold: {label: '剩余天数阈值（天）', min: 1, max: 365, tooltip: '当证书剩余天数低于此阈值时触发告警'},
    },
    {key: 'service', name: '服务下线', duration: {tooltip: '服务持续离线多久后触发告警'}},
    {key: 'agentOffline', name: '探针离线', duration: {tooltip: '探针持续离线多久后触发告警'}},
];

// 新建告警规则时的默认阈值（默认全部停用，由用户按需开启）
export const DEFAULT_ALERT_RULES = {
    cpuEnabled: false,
    cpuThreshold: 80,
    cpuDuration: 300,
    memoryEnabled: false,
    memoryThreshold: 80,
    memoryDuration: 300,
    diskEnabled: false,
    diskThreshold: 85,
    diskDuration: 300,
    networkEnabled: false,
    networkThreshold: 10,
    networkDuration: 300,
    certEnabled: false,
    certThreshold: 30,
    serviceEnabled: false,
    serviceDuration: 60,
    agentOfflineEnabled: false,
    agentOfflineDuration: 60,
};

/** 7 类告警规则的开关 + 阈值 + 持续时间表单（挂在 name="rules" 下，默认规则与自定义规则共用） */
export const AlertRulesFields = () => (
    <div>
        {ALERT_RULES.map((rule) => (
            <Form.Item key={rule.key} noStyle shouldUpdate>
                {({getFieldValue}) => {
                    const enabled = getFieldValue(['rules', `${rule.key}Enabled`]);
                    return (
                        <div
                            className="flex flex-wrap items-center justify-between gap-x-8 gap-y-4 border-t border-[#e8ebf0] py-4 first:border-t-0 first:pt-0 dark:border-[#272b33]">
                            <div className="flex items-center gap-3">
                                <Form.Item noStyle name={['rules', `${rule.key}Enabled`]} valuePropName="checked">
                                    <Switch size="small"/>
                                </Form.Item>
                                <span className="text-[13px] font-medium text-[#1f2329] dark:text-[#e6e8ec]">
                                    {rule.name}
                                </span>
                            </div>
                            <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
                                {rule.threshold && (
                                    <label
                                        className="flex items-center gap-2"
                                        title={rule.threshold.tooltip}
                                    >
                                        <span className="whitespace-nowrap text-xs text-[#646a73] dark:text-[#9ba1ab]">
                                            {rule.threshold.label}
                                        </span>
                                        <Form.Item noStyle name={['rules', `${rule.key}Threshold`]}>
                                            <InputNumber
                                                min={rule.threshold.min}
                                                max={rule.threshold.max}
                                                disabled={!enabled}
                                            />
                                        </Form.Item>
                                    </label>
                                )}
                                {rule.duration && (
                                    <label
                                        className="flex items-center gap-2"
                                        title={rule.duration.tooltip}
                                    >
                                        <span className="whitespace-nowrap text-xs text-[#646a73] dark:text-[#9ba1ab]">
                                            持续时间（秒）
                                        </span>
                                        <Form.Item noStyle name={['rules', `${rule.key}Duration`]}>
                                            <InputNumber min={1} max={3600} disabled={!enabled}/>
                                        </Form.Item>
                                    </label>
                                )}
                            </div>
                        </div>
                    );
                }}
            </Form.Item>
        ))}
    </div>
);
