---
name: helloxz-zocr
description: 使用 ZOCR MCP 服务进行 OCR 文本识别（支持本地文件、链接及懒猫网盘文件）。
---

# ZOCR OCR 文本识别技能

本技能允许你通过调用 `helloxz-zocr` MCP 服务对图片中的文本进行智能识别。

## 可用工具 (Tools)

`helloxz-zocr` 服务提供了以下两个 MCP 工具，并额外提供普通 HTTP 上传接口：

1. `ocr_image_file`：对图片文件内容进行 OCR 识别。
   - **参数**：
     - `image_base64` (string, 可选)：图片文件的 base64 编码字符串（支持带 data URI 协议前缀，如 `data:image/png;base64,...`）。
     - `file_path` (string, 可选)：本地磁盘上图片的绝对路径（仅在客户端与服务端在同一台机器且共享文件系统时可用）。
   - _注意_：`image_base64` 和 `file_path` 必须提供其中一个。**在远程调用或客户端与服务端分离的场景中，请务必使用 `image_base64` 进行传参**。

2. `ocr_image_url`：对远程图片 URL 进行 OCR 识别。
   - **参数**：
     - `url` (string, 必填)：待识别图片的 HTTP(S) URL 地址。

3. `POST /ocr/upload`：普通 HTTP 上传接口，使用 `multipart/form-data` 的 `file` 字段上传图片并返回 ZOCR 原始 JSON 结果。浏览器、脚本或其他程序直接上传图片时，优先使用此接口，避免通过 MCP/AI 传输大段 Base64。

## 执行指南与规则

当用户需要进行 OCR 文本识别时，请遵循以下流程选择最合适的处理方式：

### 1. 远程图片链接 (URL)

如果用户提供的是标准的公网图片 URL 链接，请**直接调用** `ocr_image_url` 工具传入该链接。

### 2. 普通本地文件

**如果是小龙猫本地的文件，由于其所在的环境和ZOCR服务不在同一个服务中，必须参考懒猫网盘文件中的使用代理部分**

如果用户提供的是ZOCR服务本地电脑上的图片文件：

1. 如果可以直接访问 ZOCR 服务，请优先调用 `POST /ocr/upload` 上传图片文件。
2. 如果只能使用 MCP 工具，再读取图片文件内容并编码为 Base64，调用 `ocr_image_file` 的 `image_base64` 参数。Base64 必须由程序从原始文件字节生成并直接传参，禁止手工复制、重写、摘要后再还原，避免被 AI 改写字符。

### 3. 懒猫网盘文件 (LazyCat WebDAV/Netdisk)

如果用户要求识别懒猫网盘中的图片文件，必须按以下步骤执行：

#### 3.1 获取配置

从当前运行环境提供的 HERMES/TOOLS 规则中获取配置，不要硬编码示例值：

- **网盘认证**：`## 懒猫网盘访问协议` 中的 WebDAV 地址、用户名和密码。
- **代理认证**：`## 懒猫应用访问协议` 中的代理地址、代理用户名和 Token。

凡是访问 `app.*.lzcx`、`*.heiyu.space` 或其他懒猫应用入口，都必须使用上述代理服务，**严禁直连**。

#### 3.2 获取 ZOCR 上传地址

不要写死域名。优先从当前 MCP 配置里的 `helloxz-zocr` 服务 URL 推导上传地址：

- MCP URL：`https://.../mcp`
- 上传 URL：`https://.../ocr/upload`

如果当前环境使用 OpenClaw 且存在 `~/.openclaw/openclaw.json`，可以用：

```bash
jq -r '.mcp.servers | to_entries[] | select(.key | contains("zocr")) | .value.url' ~/.openclaw/openclaw.json | sed 's|/mcp/*$|/ocr/upload|'
```

#### 3.3 下载网盘文件

通过懒猫网盘 WebDAV 下载目标图片到本地临时缓存目录。下载请求必须使用代理配置。

#### 3.4 上传与识别

优先将下载好的临时图片文件通过 `POST /ocr/upload` 上传识别。上传请求同样必须使用代理配置。

#### 3.5 Base64 兜底

如果 `/ocr/upload` 不可用，才使用 MCP 工具 `ocr_image_file` 的 `image_base64` 参数。Base64 必须由程序从原始文件字节生成并直接传参；传入前至少比较原始文件 SHA256 与 Base64 解码后的 SHA256 一致。禁止让 AI 记忆、改写或分段重组 Base64 字符串。

#### 3.6 清理缓存

识别完成后，直接删除本地临时缓存文件；如果用户明确要求保留，再保留或询问。

### 4. 结果呈现

识别完成后，将最终识别到的完整文本（`full_text`）友好地呈现给用户。如果用户有特殊需要（如需要获取文本框坐标 `boxes` 或置信度分数 `scores`），可相应格式化输出。
