import {del, get, post, put} from './request';
import type {AlertRules} from './property';
import type {AlertNotifications, AlertRule} from '../types';

export interface AlertRuleListResponse {
    items: AlertRule[];
    total: number;
    pageIndex: number;
    pageSize: number;
}

export interface AlertRuleRequest {
    name: string;
    priority: number;
    enabled: boolean;
    targetType: string;  // all / agents / tags
    agentIds: string[];
    tags: string[];
    rules: AlertRules;
    channels: string[];  // 通知渠道类型列表（空 = 所有启用渠道）
    maskIP: boolean;     // 通知中是否打码 IP 地址
    notifications: AlertNotifications; // 事件通知开关
    maintenanceEnabled: boolean;   // 是否启用每日计划维护
    maintenanceStartTime: string;  // 每日计划维护开始时间（HH:mm）
    maintenanceEndTime: string;    // 每日计划维护结束时间（HH:mm）
}

// 获取告警规则列表
export const listAlertRules = (page: number = 1, pageSize: number = 100, keyword?: string) => {
    const params = new URLSearchParams();
    params.append('pageIndex', page.toString());
    params.append('pageSize', pageSize.toString());
    if (keyword) {
        params.append('keyword', keyword);
    }
    params.set('sortOrder', 'asc');
    params.set('sortField', 'priority');
    return get<AlertRuleListResponse>(`/admin/alert-rules?${params.toString()}`);
};

// 创建告警规则
export const createAlertRule = (data: AlertRuleRequest) => {
    return post<AlertRule>('/admin/alert-rules', data);
};

// 更新告警规则
export const updateAlertRule = (id: string, data: AlertRuleRequest) => {
    return put<AlertRule>(`/admin/alert-rules/${id}`, data);
};

// 删除告警规则
export const deleteAlertRule = (id: string) => {
    return del(`/admin/alert-rules/${id}`);
};

// 启用告警规则
export const enableAlertRule = (id: string) => {
    return post(`/admin/alert-rules/${id}/enable`);
};

// 停用告警规则
export const disableAlertRule = (id: string) => {
    return post(`/admin/alert-rules/${id}/disable`);
};
