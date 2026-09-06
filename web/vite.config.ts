import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

export default defineConfig({
    base: '/admin/',
    plugins: [react(), tailwindcss()],
    publicDir: false,
    resolve: {alias: {
        '@': path.resolve(__dirname, './src'),
    }},
    build: {
        outDir: path.resolve(__dirname, 'dist'),
        emptyOutDir: true,
        assetsDir: 'assets',
    },
    server: {
        port: 5174,
        proxy: {
            // 管理后台由当前 Vite 服务提供，其余路径交给 Pika 后端，
            // 这样访问开发端口的根路径时也能打开活动的公开主题。
            '^/(?!admin(?:/|$))': {
                target: 'http://localhost:8080',
                changeOrigin: true,
                ws: true,
            },
        },
    },
});
