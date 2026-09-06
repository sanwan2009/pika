import {RouterProvider} from 'react-router-dom';
import {App as AntdApp, ConfigProvider} from 'antd';
import zhCN from 'antd/locale/zh_CN';
import adminRouter from '@/router/admin';
import {ColorModeProvider} from '@/shared/contexts/ColorModeContext';

export default function AdminApp() {
    return (
        <ColorModeProvider>
            <ConfigProvider locale={zhCN}>
                <AntdApp><RouterProvider router={adminRouter}/></AntdApp>
            </ConfigProvider>
        </ColorModeProvider>
    );
}
