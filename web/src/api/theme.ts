import {del, get, post, put} from './request';

export interface ThemeInfo {
    schemaVersion: number; id: string; name: string; description?: string; version: string; author: string;
    homepage?: string; license?: string; active: boolean; official: boolean;
    compatible: boolean; compatibilityError?: string; previewUrl: string;
}
export interface ThemeInstallResult { theme: ThemeInfo; sha256: string; }

export const listThemes = async () => (await get<ThemeInfo[]>('/admin/themes')).data;
export const uploadTheme = async (file: File) => {
    const form = new FormData();
    form.append('file', file);
    return (await post<ThemeInstallResult>('/admin/themes/upload', form, {timeout: 120000})).data;
};
export const activateTheme = async (id: string) => { await put(`/admin/themes/${encodeURIComponent(id)}/activate`); };
export const deleteTheme = async (id: string) => { await del(`/admin/themes/${encodeURIComponent(id)}`); };
