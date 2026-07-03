懒猫微服的好处不言而喻——万物互联，不过目前官方提供的好玩的功能还比较有限。不过，微服的好底子让我们有了可以发挥自己想法的机会。本文将介绍如何将应用融入到懒猫微服中，并且打通网盘的右键、MCP、SKILL等功能。

> 项目中以ZOCR项目为例，ZOCR是一个开源的OCR识别工具，支持多种语言的文字识别。
> 通过将ZOCR融入懒猫微服，我们可以实现对网盘图片的OCR识别功能，在小龙猫中使用技能对网络图片或本地图片进行OCR识别，补足部分模型不支持图片文字识别的不足。
> 也可以通过编程接口对图片进行识别，或者将MCP工具接入到其他AI应用中进行使用。

## 相关应用

## 效果演示

部署前重要说明

让用户控制部署细节

ZOCR登陆后使用

ZOCR未登陆使用

网盘右键菜单打开

使用技能对懒猫网盘图片进行OCR识别

使用技能对网络图片通过MCP工具进行OCR识别

## 接入懒猫网盘文件拦截器

如果不能选择懒猫网盘中的文件，那无疑这个应用的使用体验将大打折扣。官方提供了[专题——自动拦截文件选择器](https://developer.lazycat.cloud/lazycat-file-picker-auto-intercept.html)提供了很大的帮助，直接按照文档接入，接口实现对应的功能。

```yml
# lzc-manifest.yml
application:
  injects:
    - id: open-save-chooser
      on: browser
      when:
        - /
      do:
        # /lzcapp/pkg/content/ 是lzc-build.yml中配置的contentdir目录下的需要打包的内容，需要将文档中提供的脚本下载到本地需要打包的目录中，并修正对应的地址。
        - src: file:///lzcapp/pkg/content/lazycat-injects/lzc-file-chooser-inject.js
```

## 打通网盘右键菜单

既然我们需要选择到懒猫网盘的文件，那么如果能在网盘中使用右键菜单打开的话，体验岂不是更好。

于是，我们看到了刚才接入懒猫网盘拦截器中我们“漏掉”的一部分代码。

```yml
# lzc-manifest.yml
application:
  file_handler:
    mime:
      - image/jpeg
      - image/png
      - image/bmp
      - image/webp
    actions:
      open: /?filename=/%u # %u是懒猫网盘给过来的文件的路径。
```

那么，这个是怎么工作的呢。让我们先部署一下，然后在懒猫网盘中选择一个文件，就拿`测试文件夹`下面的`DDNS.png`举例好了。当我们依次使用右键->在线应用打开->ZOCR点击后会发现客户端帮我们打开了选择应用，并且在应用日志中我们可以看到，有一个特殊的请求日志。

```bash
# 日志中出现的请求信息
/?filename=/%E6%B5%8B%E8%AF%95%E6%96%87%E4%BB%B6%E5%A4%B9%2FDDNS.png

# 让我们使用URL解码工具解码一下
/?filename=/测试文件夹/DDNS.png
```

原来，懒猫微服会将应用打开，并且将我们选择的文件`/测试文件夹/DDNS.png`通过我们配置的回调路径`/?filename=/%u`传递过来，并跳转到对应的页面。

不过问题来了，我们虽然拿到了对应的路径，但是我们需要怎么拿到这个文件呢。

这个时候我们就又需要刚才的懒猫网盘拦截器出场了。通过研究发现，懒猫网盘提供了一个超级API，所有应用地址的 `/_lzc/files/home/` 路径都可以访问到自己的网盘文件（当然是在登录状态下），而懒猫网盘拦截器就是利用了这点实现的。那么我们就可以照葫芦画瓢，也写一个直接获取文件并赋值的。

```javascript
// lazycat-injects/auto-select-file-and-submit-inject.js
const DISK_ROOT = '/_lzc/files/home';

const encodePath = (path) =>
    String(path).split('/').map(encodeURIComponent).join('/');

const basename = (path) =>
    String(path).split('/').filter(Boolean).pop() || 'file';

const waitFor = (fn, timeout = 5000) =>
    new Promise((resolve, reject) => {
    const start = Date.now();
    const timer = setInterval(() => {
        const value = fn();
        if (value) {
        clearInterval(timer);
        resolve(value);
        } else if (Date.now() - start > timeout) {
        clearInterval(timer);
        reject(new Error('file input not found'));
        }
    }, 100);
    });

const run = async () => {
    console.debug('try to get filename');

    const params = new URLSearchParams(window.location.search);
    const targetFilename = params.get('filename');
    if (!targetFilename) return;

    console.debug('get filename:', targetFilename);

    if (window.__autoFilled) return;
    window.__autoFilled = true;

    try {
    const fileInput = await waitFor(() =>
        // 这里获取上传的input元素
        document.querySelector('#fileInput')
    );

    const response = await fetch(`${DISK_ROOT}${encodePath(targetFilename)}`);
    if (!response.ok) {
        throw new Error(`fetch file failed: ${response.status}`);
    }

    const blob = await response.blob();
    const file = new File([blob], basename(targetFilename), {
        type: blob.type || 'application/octet-stream',
        lastModified: Date.now(),
    });

    const dataTransfer = new DataTransfer();
    dataTransfer.items.add(file);

    fileInput.files = dataTransfer.files;
    fileInput.dispatchEvent(new Event('input', { bubbles: true }));
    fileInput.dispatchEvent(new Event('change', { bubbles: true }));

    console.debug('select file finish.');

    // 这里点击提交按钮
    const submitBtn = document.querySelector('#submitBtn');
    if (submitBtn) {
        setTimeout(() => submitBtn.click(), 100);
    }

    console.debug('submit finish.');
    } catch (error) {
    console.error('auto select file failed:', error);
    window.__autoFilled = false;
    }
};

// 这里判断页面加载完成后才执行
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', run);
} else {
    run();
}
```

```yml
application:
  injects:
    - id: auto-select-file-and-submit
      on: browser          # 在真实浏览器环境执行，操作页面DOM
      when:
        - /?filename   # 匹配根路径，且URL带filename查询参数（值任意）
      do: 
        - src: file:///lzcapp/pkg/content/lazycat-injects/auto-select-file-and-submit-inject.js
```

好，让我们重新部署，再次尝试使用ZOCR打开懒猫网盘中的文件，可以看到已经能自动识别我们选择好的图片了。

但是，懒猫网盘拦截器已经实现了获取文件的功能，我们为什么不直接复用呢。让我们改造一下官方提供的代码，增加一个接口让我们能直接调用原生的能力。

```javascript
// lazycat-injects/lzc-file-chooser-inject.js

/*
  ...... 此处省略了其他代码
*/

// 此处为原先的代码
const installDownloadAnchorHooks = () => {
  if (HTMLAnchorElement.prototype.click.__lzcHooked) {
    return;
  }

  const hookedClick = function () {
    if (isDownloadAnchor(this)) {
      if (shouldUseNativeDownloadAnchor(this)) {
        return STATE.hooks.originalAnchorClick.call(this);
      }
      interceptDownloadAnchorSilently(this);
      return;
    }

    return STATE.hooks.originalAnchorClick.call(this);
  };
  hookedClick.__lzcHooked = true;
  HTMLAnchorElement.prototype.click = hookedClick;
};

// 新增：导出方法
const selectLazyCatInputFile = async (input, path) => {
  const target =
    typeof input === "string" ? document.querySelector(input) : input;
  if (!(target instanceof HTMLInputElement) || target.type !== "file") {
    throw new Error("LazyCat selectInputFile needs a file input.");
  }

  target.files = await createInputFiles([{ filename: path }], target);
  target.dispatchEvent(new Event("input", { bubbles: true }));
  target.dispatchEvent(new Event("change", { bubbles: true }));
  return target.files;
};

// 新增：挂载到window对象上
window.lzcFileChooser = {
  ...(window.lzcFileChooser || {}),
  selectInputFile: selectLazyCatInputFile,
};

// 此处为原先的代码
installFilePickerHooks();
installFileInputHooks();
installDownloadAnchorHooks();
```

这样就可以最大程度的精简代码了。

```javascript
// lazycat-injects/auto-select-file-and-submit-inject.js

const run = async () => {
    console.debug('src try to get filename')
    const targetFilename = new URLSearchParams(location.search).get('filename');
    if (!targetFilename || window.__autoFilled) return;

    window.__autoFilled = true;

    // 调用懒猫网盘拦截器导出的方法，将对应的文件绑定到input元素上
    await window.lzcFileChooser.selectInputFile('#fileInput', targetFilename);

    setTimeout(() => {
        // 点击提交按钮
        document.querySelector('#submitBtn')?.click();
    }, 100);
};

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', run);
} else {
    run();
}

```

让我们重新部署试一下，发现依旧能正确自动处理选好的图片。那么，我们就成功接入网盘右键菜单功能啦。

## 制作MCP

那么，既然已经是万物互联时代了，我们的AI大总管——小龙猫怎么能不接入呢。让我们先做一个通用的MCP服务，这样技能只需要让智能体调用对应的MCP服务就好了。

> 至于为什么不直接写脚本，然后让skill来调用。是因为不一定所有的智能体都支持skill，也有一部分只支持MCP。

本来是打算直接fork改代码的，但是既然作者没搞，我也不方便直接动手（如果PR没合并，后期更新就比较麻烦）。

加上目前的接口比较简单，所以我打算直接写一个中转MCP服务。再利用[进阶主题——路由规则](https://developer.lazycat.cloud/advanced-route.html)，我们就很容易让MCP服务正常跑起来，且不干扰主程序。

```yml
# lzc-manifest.yml

application:
  routes:
    - /=http://zocr:5080/
  upstreams:
    - location: /mcp # mcp入口地址
      disable_trim_location: true # 转发时不去掉location
      backend: http://127.0.0.1:8080 # string 上游的地址，需要是一个合法的url，支持http,https,file三个协议
      use_backend_host: false # 访问上游时http host header使用backend中的host，而非浏览器请求时的host
      backend_launch_command: ZOCR_API_URL=http://zocr:5080 ZOCR_TOKEN={{ index .UserParams "zocr.token" }} /lzcapp/pkg/content/mcp-server/mcp-server # string 自动启动此字段里的程序
```

不过有个小插曲，由于MCP并不支持文件传输了，所以图片内容需要进行base64后再通过MCP进行识别。

但是这里有个坑，大模型的记忆并不可靠，这也就导致我最初测试的十几次都是错误的base64信息，然后程序会解析图片失败，并且反响解码后的图片也不是原图。所以，需要文件传输的接口还是要单独设计，然后在MCP中明确该接口的用法即可。

当然，最后不要忘记导出你的MCP，不然小龙猫可是不会给你干活的哦。

```yml
# lzc-build.yml

resource_exports:
  - kind: mcp-providers
    source: ./resources/mcp-providers/

```

## 制作SKILL

skill的话，懂得都懂，直接按照MCP或者接口的使用方式写如何使用就行了，但是不要忘记在配置中增加技能导出。

```yml
# lzc-build.yml

resource_exports:
  - kind: skills
    source: ./resources/skills/

```

Q：那么让我们部署测试一下吧，但是为什么这么不稳定呢，好想http接口找不到对应的地址，就算找到也会失败。
A：这是因为[进阶主题——应用间访问](https://developer.lazycat.cloud/advanced-app-interconnect.html)中有提到，与其他应用访问时，可以使用内部接口`app.<target-pkg-id>.lzcx`，但是需要请求时在`header`中增加`X-HC-USER-TICKET`鉴权。

Q：但是小龙猫哪里知道这些，我岂不是要写很麻烦的说明文档吗，况且我也拿不到这个凭据啊。
A：不要着急，小龙猫设计的时候就已经规划好了，在`懒猫网盘: 全部文件/应用文稿/小龙猫/{你的龙猫}/当前记忆/workspace/TOOLS.md`中已经写好了，如果要访问懒猫应用地址是，可以增加对应的代理。而我们只需要在技能中明确说明，这个额外的HTTP接口你也要使用这个代理就可以啦。

当然这里说的比较泛一点，有不明白的小伙伴可以去我移植的ZOCR项目中查看我写的技能提示词，或者让你的小龙猫看一下其他技能中是否有出现过类似的提示词，并让它给你生成一个新的，接着就是反复部署，开新的回话进行测试调整。

## 知识点巩固

1. 文件夹和权限(系统版本V1.6.0)

| 文件夹 | 对应权限 | 功能 | 发展 | 
|:-|:-|:-|:-|
| /lzcapp/var |  自动挂载 | 存储应用数据，卸载时可选删除，在懒猫网盘/应用数据中查看 | - |
| /lzcapp/cache | 自动挂载 | 应用缓存目录，不重要可随时清理的内容，目前直接在网盘中查看 | - |
| /lzcapp/run/mnt/home | 自动挂载 | **被废弃的**用户文稿目录 | V1.7.0后需要管理员明确授权 **尽量不要使用** |
| /lzcapp/document | document.read/document.write | 新的用户文稿目录 | V1.7.0后正式上线 |
| /lzcapp/documents | document.private | 应用文稿根目录，可在懒猫网盘/全部文件/应用文稿中查看 | 推荐使用，应用卸载后默认不会删除 |

其他参考：[最佳实践——文件访问](https://developer.lazycat.cloud/advanced-file.html)

2. 路由权限和访问

- 同一个应用不同的服务访问使用域名`$service_name.$appid.lzcapp`，详见[进阶主题——路由规则](https://developer.lazycat.cloud/advanced-route.html#http%E4%B8%8A%E6%B8%B8)
- 不同应用间访问使用域名`app.<target-pkg-id>.lzcx`，并需要`X-HC-USER-TICKET`鉴权，详见[进阶主题——应用间访问](https://developer.lazycat.cloud/advanced-app-interconnect.html)
- 无需登陆微服即可访问服务，适用于服务器上内网穿透公开服务（不建议），详见[进阶主题——独立鉴权](https://developer.lazycat.cloud/advanced-public-api.html)
- 登陆状态下才能使用懒猫网盘拦截器，详见[进阶主题——脚本注入](https://developer.lazycat.cloud/advanced-injects.html)

## 最后说两句

看似简单的功能，但其实设计的知识点还是很多的。尽管目前已经对接了很多，但是还有客户端的部分也可以想办法接入。不过还在现在流程还能跑通，这也多亏了懒猫微服的设计者在背后做了很多的工作。不过随着技术的迭代，新的内容和挑战也一直出现。让我们一起学习，打造属于我们自己的数据中心和“小龙猫”吧。

> 最后多嘴一句，不知道我之前提到的[ActivityPub](https://zh.wikipedia.org/wiki/ActivityPub)宇宙，会不会在工程师的规划之内呢。

## 参考链接

- [fghwett/helloxz-zocr-lzcapp](https://github.com/fghwett/helloxz-zocr-lzcapp)
- [懒猫微服开发者手册](https://developer.lazycat.cloud/)

## 特别鸣谢

- [ZOCR项目](https://github.com/helloxz/zocr)
- [懒猫微服应用商店官方支持团队](https://lazycat.cloud/about?navtype=AfterSalesService)
