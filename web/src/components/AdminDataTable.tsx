import type {TableProps} from 'antd';
import {Table, theme} from 'antd';

/** 管理列表共用的数据表格：紧凑、直角，并跟随 Ant Design 主题背景。 */
export const AdminDataTable = <RecordType extends object>(props: TableProps<RecordType>) => {
    const {token} = theme.useToken();

    return (
        <div className="min-w-0 overflow-hidden bg-white dark:bg-[#171a21] p-4 rounded-md">
            <Table<RecordType>
                size="small"
                {...props}
                styles={{
                    root: {borderRadius: 0, background: token.colorBgContainer},
                    content: {borderRadius: 0, background: token.colorBgContainer},
                    ...props.styles,
                }}
            />
        </div>
    );
};
