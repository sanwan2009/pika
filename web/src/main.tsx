import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import AdminApp from './App';
import './index.css';

dayjs.locale('zh-cn');

const queryClient = new QueryClient({
    defaultOptions: {queries: {refetchOnWindowFocus: false, retry: 1}},
});

createRoot(document.getElementById('root')!).render(
    <StrictMode>
        <QueryClientProvider client={queryClient}>
            <AdminApp/>
        </QueryClientProvider>
    </StrictMode>,
);
