import type {PropsWithChildren, ReactNode} from 'react';
import {cn} from '@/lib/utils';

interface SettingsSectionProps {
    title: string;
    description?: ReactNode;
    /** 渲染与上一区块的分隔线，首个区块传 false */
    divided?: boolean;
    className?: string;
    /** 标题行右侧内容（如操作按钮） */
    extra?: ReactNode;
    children: ReactNode;
}

/** 设置页内部的平铺分区：标题 + 描述 + 内容，用分隔线取代嵌套卡片。 */
export const SettingsSection = ({title, description, divided = true, className, extra, children}: SettingsSectionProps) => (
    <section className={cn(divided && 'mt-6 border-t border-[#e8ebf0] pt-5 dark:border-[#272b33]', className)}>
        <div className="flex min-w-0 items-center justify-between gap-3">
            <h3 className="m-0 text-sm font-semibold text-[#1f2329] dark:text-[#e6e8ec]">{title}</h3>
            {extra ? <div className="shrink-0">{extra}</div> : null}
        </div>
        {description ? (
            <p className="mt-1 mb-0 text-xs text-[#646a73] dark:text-[#9ba1ab]">{description}</p>
        ) : null}
        <div className="mt-4">{children}</div>
    </section>
);

/** 设置页底部的操作按钮行。 */
export const SettingsActions = ({children}: PropsWithChildren) => (
    <div className="mt-6 flex flex-wrap items-center gap-3 border-t border-[#e8ebf0] pt-4 dark:border-[#272b33]">
        {children}
    </div>
);

interface SettingsSwitchRowProps {
    title: string;
    description?: ReactNode;
    /** 右侧的开关（通常是 noStyle 的 Form.Item + Switch） */
    children: ReactNode;
}

/** 带说明文字的开关行：左侧标题与描述，右侧开关。 */
export const SettingsSwitchRow = ({title, description, children}: SettingsSwitchRowProps) => (
    <div className="flex items-center justify-between gap-4 border-t border-[#e8ebf0] py-3.5 first:border-t-0 first:pt-0 dark:border-[#272b33]">
        <div className="min-w-0">
            <div className="text-[13px] font-medium text-[#1f2329] dark:text-[#e6e8ec]">{title}</div>
            {description ? (
                <div className="mt-0.5 text-xs text-[#646a73] dark:text-[#9ba1ab]">{description}</div>
            ) : null}
        </div>
        <div className="shrink-0">{children}</div>
    </div>
);
