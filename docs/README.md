# Gemini Web To API — 接入指南

## 目录

- [1. 编译与部署](#1-编译与部署)
  - [1.1 环境准备](#11-环境准备)
  - [1.2 编译](#12-编译)
  - [1.3 配置](#13-配置)
  - [1.4 启动](#14-启动)
  - [1.5 systemd 托管](#15-systemd-托管)
  - [1.6 nginx 反代 + HTTPS](#16-nginx-反代--https)
  - [1.7 可用模型](#17-可用模型)
- [2. 各客户端接入方式](#2-各客户端接入方式)
  - [2.1 Claude Code](#21-claude-code)
  - [2.2 OpenAI SDK / 兼容客户端](#22-openai-sdk--兼容客户端)
  - [2.3 Claude API / Anthropic SDK](#23-claude-api--anthropic-sdk)
  - [2.4 Gemini 原生 SDK](#24-gemini-原生-sdk)
  - [2.5 LangChain](#25-langchain)
  - [2.6 其他兼容 OpenAI 的工具](#26-其他兼容-openai-的工具)
- [3. 运维](#3-运维)
- [4. 安全建议](#4-安全建议)

---

## 1. 编译与部署

### 1.1 环境准备

- Go 1.23+（推荐 1.25+）
- Linux 服务器（Ubuntu 22.04+ / Debian）
- nginx（可选，用于反代 + HTTPS）
- 一个域名 + SSL 证书（可选，推荐 Let's Encrypt + acme.sh）

### 1.2 编译

```bash
git clone https://github.com/xiaoyekanren/fork-gemini-web-to-api.git
cd gemini-web-to-api
go build -o gemini-web-to-api ./cmd/server/
```

交叉编译 Linux 二进制（在 macOS/Windows 上）：

```bash
GOOS=linux GOARCH=amd64 go build -o gemini-web-to-api ./cmd/server/
```

### 1.3 配置

在项目根目录创建 `.env` 文件：

```env
# 必填 — Google Cookie
GEMINI_COOKIES='APISID=...; SAPISID=...; __Secure-1PSID=...; __Secure-1PSIDCC=...'

# 必填 — API 鉴权密钥（未设置则跳过鉴权，等同裸奔）
API_KEY=openssl-rand-hex-32-生成的随机字符串

# 可选 — 默认值如下
GEMINI_MAX_RETRIES=3
PORT=4981
HOST=127.0.0.1
RATE_LIMIT_ENABLED=true
RATE_LIMIT_WINDOW_MS=60000
RATE_LIMIT_MAX_REQUESTS=20
```

**获取 Cookie**：

推荐从浏览器 Network 请求头复制完整 Cookie 字符串，避免从 Cookies 面板逐个复制时漏项。

1. 用 Chrome/Edge 访问 https://gemini.google.com，确认已登录且能正常聊天
2. 按 `F12` 打开开发者工具，进入 `Network` / `网络`
3. 刷新页面，或随便发送一句话
4. 点开一个发往 `gemini.google.com` 的请求，优先找 `StreamGenerate`、`batchexecute` 或任意 Gemini 请求
5. 在 `Headers` / `标头` → `Request Headers` / `请求标头` 中找到 `Cookie:`
6. 复制 `Cookie:` 后面的完整内容，填入 `GEMINI_COOKIES`

`GEMINI_COOKIES` 必须是 `name=value; name=value` 形式，例如：

```env
GEMINI_COOKIES='APISID=...; SAPISID=...; SID=...; __Secure-1PSID=...; __Secure-1PSIDCC=...; __Secure-3PSID=...'
```

不要包含 `Cookie:` 这几个字，也不要只复制单个 value。建议用单引号包裹整串 Cookie，因为浏览器 Cookie 里可能包含 `$`，双引号在 shell 或 dotenv 场景下可能触发变量展开。`GEMINI_1PSID` / `GEMINI_1PSIDCC` 仍可作为兼容快捷项，但推荐使用完整 Cookie 集。

> [!TIP]
> 如果服务器需要通过代理访问外网，在 `.env` 中额外添加：
> ```env
> HTTP_PROXY=http://127.0.0.1:7890
> HTTPS_PROXY=http://127.0.0.1:7890
> ```
> 否则 Gemini Web API 的连接会超时卡在 SYN-SENT。

环境变量说明：

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `GEMINI_COOKIES` | 是 | — | 已登录 Gemini 浏览器会话的完整 Cookie 集 |
| `GEMINI_1PSID` | 否 | — | 兼容快捷项；存在 `GEMINI_COOKIES` 时会从中解析 |
| `GEMINI_1PSIDCC` | 否 | — | 兼容快捷项；存在 `GEMINI_COOKIES` 时会从中解析 |
| `API_KEY` | 建议 | — | API 鉴权密钥，留空则不校验 |
| `GEMINI_MAX_RETRIES` | 否 | `3` | API 调用失败最大重试次数 |
| `PORT` | 否 | `4981` | 监听端口 |
| `HOST` | 否 | 空（`0.0.0.0`） | 绑定地址，反代场景设 `127.0.0.1` |
| `RATE_LIMIT_ENABLED` | 否 | `false` | 是否开启限流 |
| `RATE_LIMIT_WINDOW_MS` | 否 | `60000` | 限流时间窗口（毫秒） |
| `RATE_LIMIT_MAX_REQUESTS` | 否 | `10` | 窗口内最大请求数 |

### 1.4 启动

直接启动（开发/调试）：

```bash
cd /opt/gemini-web-to-api
./gemini-web-to-api
```

### 1.5 systemd 托管

创建 `/etc/systemd/system/gemini-web-to-api.service`：

```ini
[Unit]
Description=Gemini Web To API
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/gemini-web-to-api
Environment="HOST=127.0.0.1"
EnvironmentFile=/opt/gemini-web-to-api/.env
ExecStart=/opt/gemini-web-to-api/gemini-web-to-api
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gemini-web-to-api

# 如果你的服务器需要代理才能访问 Gemini（国内常见），
# 取消下面三行的注释并填上你的代理地址：
# Environment="HTTP_PROXY=http://127.0.0.1:7890"
# Environment="HTTPS_PROXY=http://127.0.0.1:7890"
# Environment="NO_PROXY=localhost,127.0.0.1,.local"

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable gemini-web-to-api
sudo systemctl start gemini-web-to-api
sudo systemctl status gemini-web-to-api
```

### 1.6 nginx 反代 + HTTPS

以 `api.example.com` 为例，nginx 配置（`/usr/local/nginx/conf/user_defined_config/gemini-api.conf` 或 `/etc/nginx/sites-enabled/gemini-api`）：

```nginx
server {
  listen 80;
  server_name api.example.com;
  return 301 https://$host$request_uri;
}

server {
  listen 443 ssl;
  server_name api.example.com;

  ssl_certificate         /path/to/fullchain.cer;
  ssl_certificate_key     /path/to/domain.key;
  ssl_protocols           TLSv1.2 TLSv1.3;
  ssl_ciphers ECDHE-RSA-AES256-SHA384:AES256-SHA256:RC4:HIGH:!MD5:!aNULL:!eNULL:!NULL:!DH:!EDH:!AESGCM;
  ssl_session_timeout 10m;
  ssl_prefer_server_ciphers on;
  ssl_session_cache shared:SSL:10m;

  location / {
    proxy_pass http://127.0.0.1:4981;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # SSE 流式必须
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
    chunked_transfer_encoding on;
  }
}
```

验证并重载：

```bash
sudo nginx -t && sudo nginx -s reload
```

> 健康检查 `/health` 端点无需 API Key，可配入监控或负载均衡探测。

### 1.7 可用模型

启动后可通过 `GET /openai/v1/models` 查看完整模型列表。下面按用途分类说明。

#### 当前主力（推荐首选）

| 模型 ID | 说明 |
|----------|------|
| `gemini-3.5-flash` | 最新 Flash 模型，响应快、成本低，适合日常对话和轻量任务 |
| `gemini-3.1-pro` | 最新 Pro 模型，推理和编程能力最强，适合复杂分析、长文本 |
| `gemini-advanced` | Gemini Advanced 别名（即 2.5 Pro），稳定版 Pro，兼容性最好 |

> **建议：** 日常用 `gemini-3.5-flash`，复杂任务用 `gemini-3.1-pro`。Claude Code 等工具推荐配 `gemini-3.1-pro` 或 `gemini-advanced`。

#### 预览版

| 模型 ID | 说明 |
|----------|------|
| `gemini-3-flash-preview` | 3.x Flash 系列预览，抢先体验新特性 |
| `gemini-2.5-flash` | 2.5 Flash 稳定版，上一代主力 Flash |
| `gemini-2.5-flash-preview-05-20` | 2.5 Flash 5/20 预览快照 |
| `gemini-2.5-flash-preview-04-17` | 2.5 Flash 4/17 预览快照 |
| `gemini-2.5-flash-preview-09-2025` | 2.5 Flash 9月预览快照 |

#### 图片 / 多模态

| 模型 ID | 说明 |
|----------|------|
| `gemini-3.1-flash-image-preview` | 3.1 Flash 图片生成预览 |
| `gemini-3-pro-image-preview-11-2025` | 3 Pro 图片生成预览（2025/11 快照） |
| `gemini-2.5-flash-image-preview` | 2.5 Flash 图片生成预览 |
| `gemini-2.5-flash-image` | 2.5 Flash 图片生成 |

#### 其他

| 模型 ID | 说明 |
|----------|------|
| `gemini-2.5-flash-preview-tts` | 2.5 Flash TTS 语音合成预览 |
| `gemini-2.0-flash` | 旧版 2.0 Flash，不推荐新项目使用 |

> 列表中还有 `gemini-apps-while-signed-out`、`gemini-u-top-priority-*`、`gemini-u-lesson-tile-hover-bg` 等为 Google 内部/UI 使用的模型，非对话用途，忽略即可。

---

## 2. 各客户端接入方式

> 下文中 `${BASE_URL}` 替换为实际部署地址，如 `https://api.example.com`。
> `<YOUR_API_KEY>` 替换为 `.env` 中 `API_KEY` 的值。

### 2.1 Claude Code

编辑 `~/.claude/settings.json`（Windows 路径 `C:\Users\<用户名>\.claude\settings.json`）：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "${BASE_URL}/claude",
    "ANTHROPIC_AUTH_TOKEN": "<YOUR_API_KEY>",
    "ANTHROPIC_MODEL": "gemini-3.1-pro",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "gemini-3.1-pro",
    "ANTHROPIC_DEFAULT_OPUS_MODEL_NAME": "gemini-3.1-pro",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "gemini-3.5-flash",
    "ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "gemini-3.5-flash",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gemini-3.5-flash",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME": "gemini-3.5-flash"
  }
}
```

> Claude Code 使用 Anthropic 协议。`ANTHROPIC_BASE_URL` 指向 `/claude`，SDK 自动拼接 `/v1/messages`。`ANTHROPIC_AUTH_TOKEN` 以 `x-api-key` header 发送。

---

### 2.2 OpenAI SDK / 兼容客户端

| 参数 | 值 |
|------|----|
| base_url | `${BASE_URL}/openai/v1` |
| api_key | `<YOUR_API_KEY>` |
| model | `gemini-3.1-pro` 或 `gemini-3.5-flash` |

可用模型列表：`GET ${BASE_URL}/openai/v1/models`

**Python 示例：**

```python
from openai import OpenAI

client = OpenAI(
    base_url="${BASE_URL}/openai/v1",
    api_key="<YOUR_API_KEY>"
)

# 非流式
response = client.chat.completions.create(
    model="gemini-3.1-pro",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)

# 流式
stream = client.chat.completions.create(
    model="gemini-3.1-pro",
    messages=[{"role": "user", "content": "写一首诗"}],
    stream=True
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

**Node.js 示例：**

```js
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "${BASE_URL}/openai/v1",
  apiKey: "<YOUR_API_KEY>",
});

const response = await client.chat.completions.create({
  model: "gemini-3.1-pro",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response.choices[0].message.content);
```

**cURL 示例：**

```bash
curl -X POST ${BASE_URL}/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-api-key: <YOUR_API_KEY>" \
  -d '{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"Hello!"}]}'
```

**兼容 OpenAI API 的桌面/Web 客户端**（ChatBox、NextChat、OpenCat、CodeGPT、Lobechat 等），在设置中修改 API 地址为 `${BASE_URL}`，路径为 `/openai/v1/chat/completions`，填入 API Key 即可。

---

### 2.3 Claude API / Anthropic SDK

| 参数 | 值 |
|------|----|
| base_url | `${BASE_URL}/claude` |
| api_key | `<YOUR_API_KEY>` |

**Python 示例：**

```python
import anthropic

client = anthropic.Anthropic(
    base_url="${BASE_URL}/claude",
    api_key="<YOUR_API_KEY>"
)

response = client.messages.create(
    model="gemini-3.1-pro",
    max_tokens=4096,
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.content[0].text)
```

**流式：**

```python
with client.messages.stream(
    model="gemini-3.1-pro",
    max_tokens=4096,
    messages=[{"role": "user", "content": "Hello!"}]
) as stream:
    for text in stream.text_stream:
        print(text, end="")
```

**cURL 示例：**

```bash
curl -X POST ${BASE_URL}/claude/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: <YOUR_API_KEY>" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"gemini-3.1-pro","max_tokens":4096,"messages":[{"role":"user","content":"Hello!"}]}'
```

---

### 2.4 Gemini 原生 SDK

```python
import google.generativeai as genai

genai.configure(
    api_key="<YOUR_API_KEY>",
    transport="rest",
    client_options={"api_endpoint": "${BASE_URL}/gemini"}
)

model = genai.GenerativeModel("gemini-3.1-pro")
response = model.generate_content("Write a poem about coding")
print(response.text)
```

---

### 2.5 LangChain

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="${BASE_URL}/openai/v1",
    api_key="<YOUR_API_KEY>",
    model="gemini-3.1-pro"
)

response = llm.invoke("Explain quantum computing")
print(response.content)
```

---

### 2.6 其他兼容 OpenAI 的工具

以下工具在设置中将 API 地址改为 `${BASE_URL}/openai/v1`，API Key 填 `<YOUR_API_KEY>` 即可：

| 工具 | 说明 |
|------|------|
| Aider | AI 结对编程 CLI |
| Open Interpreter | 自然语言编程工具 |
| Continue.dev | VS Code / JetBrains AI 插件 |
| Cursor | AI 编辑器（支持 OpenAI 兼容） |
| ChatBox | 桌面 AI 客户端 |
| OpenCat | macOS AI 客户端 |
| Lobechat | Web AI 客户端 |

---

## 3. 运维

### 查看日志

```bash
sudo journalctl -u gemini-web-to-api -f
```

### 重启服务

```bash
sudo systemctl restart gemini-web-to-api
```

### 更新代码

```bash
cd /opt/gemini-web-to-api
git pull origin main
/usr/local/go/bin/go build -o gemini-web-to-api ./cmd/server/
sudo systemctl restart gemini-web-to-api
```

### 查看可用模型

```bash
curl -H "x-api-key: <YOUR_API_KEY>" ${BASE_URL}/openai/v1/models
```

### 健康检查

```bash
curl ${BASE_URL}/health
```

---

## 4. 安全建议

- **API_KEY 务必设强随机值**：`openssl rand -hex 32`
- **Cookie 保密**：`.env` 中的 `GEMINI_COOKIES` 等同于 Google 账号密码，切勿泄露或提交到 Git
- **绑定 localhost**：使用 nginx 反代时设 `HOST=127.0.0.1`，避免端口直接暴露
- **限流保护**：开启 `RATE_LIMIT_ENABLED=true`，按需调整阈值
- **只允许自己用**：可在 nginx 中加 IP 白名单或启用 `satisfy any` + HTTP Basic Auth
