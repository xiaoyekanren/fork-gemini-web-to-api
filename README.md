# AI Bridges 🚀

**AI Bridges** is a high-performance WebAI-to-API service built in Go. It allows you to convert web-based AI services (like Google Gemini) into standardized REST APIs, including an OpenAI-compatible interface.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## ✨ Features

- 🌉 **Service Bridge**: Seamlessly connect web-based AI to your applications.
- 🤖 **Gemini Support**: Full support for Google Gemini (pro) using session cookies.
- 🔄 **Auto Cookie Rotation**: Automatically manages and refreshes session tokens (`GEMINI_1PSIDTS`) to keep the connection alive.
- 🔌 **OpenAI Compatible**: Provides an `/v1/chat/completions` endpoint that mimics OpenAI's API.
- 🚀 **Built with Fiber**: Ultra-fast and efficient web framework.
- 📝 **Swagger UI**: Interactive API documentation built-in.
- 🐳 **Dockerized**: Ready for containerized deployment with unified configuration.

---

## 🛠️ Technology Stack

- **Language**: Go (v1.24+)
- **Framework**: [Gofiber/fiber](https://github.com/gofiber/fiber)
- **HTTP Client**: [req/v3](https://github.com/imroc/req/v3)
- **Logging**: Uber-zap
- **Documentation**: Swag / Swagger

---

## 🚀 Getting Started

### Prerequisites

- Go 1.24 or higher installed. Or Docker.

### Configuration Priority

The application uses a unified configuration system with the following priority:

1. **Environment Variables** (Highest priority)
2. **`config.yml`**
3. **Defaults** (Lowest priority)

### Environment Variables

| Variable                  | Corresponding YAML Key    | Description                               |
| ------------------------- | ------------------------- | ----------------------------------------- |
| `GEMINI_1PSID`            | `GEMINI_1PSID`            | (Required) Main session cookie            |
| `GEMINI_1PSIDTS`          | `GEMINI_1PSIDTS`          | (Highly Recommended) Timestamp cookie     |
| `GEMINI_1PSIDCC`          | `GEMINI_1PSIDCC`          | (Optional) Context cookie                 |
| `GEMINI_REFRESH_INTERVAL` | `GEMINI_REFRESH_INTERVAL` | Rotation interval in minutes (default: 5) |
| `PORT`                    | `PORT`                    | Server port (default: 3000)               |

---

## 🐳 Docker Usage

### Docker Compose (Recommended)

Create a `docker-compose.yml`:

```yaml
services:
  ai-bridges:
    build: .
    ports:
      - "3000:3000"
    environment:
      - PORT=3000
      - GEMINI_1PSID=your_psid_here
      - GEMINI_1PSIDTS=your_psidts_here
      - GEMINI_1PSIDCC=your_psidcc_here
      - GEMINI_REFRESH_INTERVAL=30
    restart: always
```

### Direct Docker Run

```bash
docker run -p 3000:3000 \
  -e GEMINI_1PSID="your_psid_here" \
  -e GEMINI_1PSIDTS="your_psidts_here" \
  -e GEMINI_1PSIDCC="your_psidcc_here" \
  -e GEMINI_REFRESH_INTERVAL=30 \
  -e PORT=3000 \
  ai-bridges
```

---

## 🧪 Quick Testing

Once the server is running, you can test the Gemini connection using the following command:

### Test Gemini Chat

```bash
curl -X 'POST' \
  'http://localhost:3000/gemini/chat' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "message": "Who are you?"
}'
```

### Test OpenAI Compatible Endpoint

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-pro",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## 📘 API Documentation

Visit `http://localhost:3000/swagger/` for the full interactive API documentation.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
