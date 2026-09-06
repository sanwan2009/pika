import {del, get, post, put} from './request';
import type {CreateDDNSConfigRequest, DDNSConfig, DDNSRecord, UpdateDDNSConfigRequest} from '@/types/ddns';

export interface DDNSConfigListResponse {
    items: DDNSConfig[];
    total: number;
}

export interface DDNSRecordListResponse {
    items: DDNSRecord[];
    total: number;
}

// 获取 DDNS 配置列表（分页）
export const getDDNSConfigs = (pageIndex: number, pageSize: number, name?: string, agentId?: string) => {
    const params = new URLSearchParams();
    params.append('pageIndex', pageIndex.toString());
    params.append('pageSize', pageSize.toString());
    if (name) {
        params.append('name', name);
    }
    if (agentId) {
        params.append('agentId', agentId);
    }
    return get<DDNSConfigListResponse>(`/admin/ddns?${params.toString()}`);
};

// 创建 DDNS 配置
export const createDDNSConfig = (data: CreateDDNSConfigRequest) => {
    return post<DDNSConfig>('/admin/ddns', data);
};

// 获取 DDNS 配置详情
export const getDDNSConfig = (id: string) => {
    return get<DDNSConfig>(`/admin/ddns/${id}`);
};

// 更新 DDNS 配置
export const updateDDNSConfig = (id: string, data: UpdateDDNSConfigRequest) => {
    return put<{ message: string }>(`/admin/ddns/${id}`, data);
};

// 删除 DDNS 配置
export const deleteDDNSConfig = (id: string) => {
    return del<{ message: string }>(`/admin/ddns/${id}`);
};

// 启用 DDNS 配置
export const enableDDNSConfig = (id: string) => {
    return post<{ message: string }>(`/admin/ddns/${id}/enable`, {});
};

// 禁用 DDNS 配置
export const disableDDNSConfig = (id: string) => {
    return post<{ message: string }>(`/admin/ddns/${id}/disable`, {});
};

// 获取 DDNS 更新记录
export const getDDNSRecords = (id: string) => {
    return get<DDNSRecordListResponse>(`/admin/ddns/${id}/records`);
};

// 手动触发 DDNS 更新
export const triggerDDNSUpdate = (id: string) => {
    return post<{ message: string }>(`/admin/ddns/${id}/trigger`, {});
};
