(function () {
    const run = async () => {
        console.debug('src try to get filename')
        const targetFilename = new URLSearchParams(location.search).get('filename');
        if (!targetFilename || window.__autoFilled) return;

        window.__autoFilled = true;

        await window.lzcFileChooser.selectInputFile('#fileInput', targetFilename);

        setTimeout(() => {
        document.querySelector('#submitBtn')?.click();
        }, 100);
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', run);
    } else {
        run();
    }

    // const DISK_ROOT = '/_lzc/files/home';

    // const encodePath = (path) =>
    //     String(path).split('/').map(encodeURIComponent).join('/');

    // const basename = (path) =>
    //     String(path).split('/').filter(Boolean).pop() || 'file';

    // const waitFor = (fn, timeout = 5000) =>
    //     new Promise((resolve, reject) => {
    //     const start = Date.now();
    //     const timer = setInterval(() => {
    //         const value = fn();
    //         if (value) {
    //         clearInterval(timer);
    //         resolve(value);
    //         } else if (Date.now() - start > timeout) {
    //         clearInterval(timer);
    //         reject(new Error('file input not found'));
    //         }
    //     }, 100);
    //     });

    // const run = async () => {
    //     console.debug('try to get filename');

    //     const params = new URLSearchParams(window.location.search);
    //     const targetFilename = params.get('filename');
    //     if (!targetFilename) return;

    //     console.debug('get filename:', targetFilename);

    //     if (window.__autoFilled) return;
    //     window.__autoFilled = true;

    //     try {
    //     const fileInput = await waitFor(() =>
    //         document.querySelector('#fileInput')
    //     );

    //     const response = await fetch(`${DISK_ROOT}${encodePath(targetFilename)}`);
    //     if (!response.ok) {
    //         throw new Error(`fetch file failed: ${response.status}`);
    //     }

    //     const blob = await response.blob();
    //     const file = new File([blob], basename(targetFilename), {
    //         type: blob.type || 'application/octet-stream',
    //         lastModified: Date.now(),
    //     });

    //     const dataTransfer = new DataTransfer();
    //     dataTransfer.items.add(file);

    //     fileInput.files = dataTransfer.files;
    //     fileInput.dispatchEvent(new Event('input', { bubbles: true }));
    //     fileInput.dispatchEvent(new Event('change', { bubbles: true }));

    //     console.debug('select file finish.');

    //     const submitBtn = document.querySelector('#submitBtn');
    //     if (submitBtn) {
    //         setTimeout(() => submitBtn.click(), 100);
    //     }

    //     console.debug('submit finish.');
    //     } catch (error) {
    //     console.error('auto select file failed:', error);
    //     window.__autoFilled = false;
    //     }
    // };

    // if (document.readyState === 'loading') {
    //     document.addEventListener('DOMContentLoaded', run);
    // } else {
    //     run();
    // }

    // // 等待DOM加载完成后执行
    // const run = () => {
    //     console.debug('try to get filename');
    //     // 1. 从URL中读取filename参数
    //     const params = new URLSearchParams(window.location.search);
    //     const targetFilename = params.get('filename');
    //     if (!targetFilename) return;
    //     console.debug('get filename:', targetFilename);

    //     // 防止重复执行，避免页面重渲染反复触发
    //     if (window.__autoFilled) return;
    //     window.__autoFilled = true;

    //     // 2. 选中编辑器中的对应文件
    //     // 注意：请将下面的选择器替换为你页面中实际的文件选择器
    //     const fileSelect = document.querySelector('#fileInput'); 
    //     if (fileSelect) {
    //     fileSelect.value = targetFilename;
    //     // 触发change事件，确保React/Vue等前端框架感知到值变化
    //     fileSelect.dispatchEvent(new Event('change', { bubbles: true }));
    //     }
    //     console.debug('select file finish.')

    //     // 3. 自动点击提交按钮
    //     const submitBtn = document.querySelector('#submitBtn');
    //     if (submitBtn) {
    //     // 可选：加短延迟，确保编辑器状态更新完成后再提交
    //     setTimeout(() => submitBtn.click(), 100);
    //     }
    //     console.debug('submit finish.')
    // };

    // // DOM未解析完成则等待加载，已完成则直接执行
    // if (document.readyState === 'loading') {
    //     document.addEventListener('DOMContentLoaded', run);
    // } else {
    //     run();
    // }
})()