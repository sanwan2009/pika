import React from 'react';
import {Button} from 'antd';
import type {LucideIcon} from 'lucide-react';

export interface Action {
    key: string;
    label: string;
    icon?: React.ReactElement<LucideIcon>;
    type?: 'default' | 'primary';
    onClick: () => void;
    danger?: boolean;
    disabled?: boolean;
    loading?: boolean;
}

interface PageHeaderProps {
    title: React.ReactNode;
    extra?: React.ReactNode;
    actions?: Action[];
}

/**
 * 统一的页面头部组件
 */
export const PageHeader: React.FC<PageHeaderProps> = ({title, extra, actions}) => {
    return (
        <div className="flex min-h-[42px] flex-wrap items-center justify-between gap-3 max-sm:items-start">
            <div className="max-sm:w-full">
                <h1 className="m-0 text-xl leading-tight font-semibold tracking-[-0.025em] text-[#1f2329] dark:text-[#e6e8ec] max-sm:pt-1.5 max-sm:text-[19px]">{title}</h1>
            </div>
            {(extra || (actions && actions.length > 0)) && (
                <div className="flex flex-wrap items-center justify-end gap-[7px] max-sm:w-full max-sm:justify-start max-sm:gap-1.5">
                    {extra}
                    {actions?.map((action) => (
                        <div key={action.key} className="shrink-0">
                            <Button
                                type={action.type || 'default'}
                                icon={action.icon}
                                onClick={action.onClick}
                                danger={action.danger}
                                disabled={action.disabled}
                                loading={action.loading}
                            >
                                {action.label}
                            </Button>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};
