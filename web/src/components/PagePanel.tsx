import type {HTMLAttributes, PropsWithChildren} from 'react';
import {cn} from '@/lib/utils';

type PagePanelProps = PropsWithChildren<HTMLAttributes<HTMLDivElement>>;

/** 一级页面使用的统一内容面板。 */
export const PagePanel = ({children, className, ...props}: PagePanelProps) => (
    <section className={cn(
        'overflow-hidden rounded-[10px] border border-[#e8ebf0] bg-white p-[18px] dark:border-[#272b33] dark:bg-[#171a21] max-sm:rounded-[13px] max-sm:p-3.5',
        className,
    )} {...props}>
        {children}
    </section>
);
