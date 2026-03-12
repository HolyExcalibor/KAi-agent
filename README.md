# KAI Agent

A Go-based AI agent powered by OpenAI with built-in tools and persistent memory.

## Features

- 🤖 OpenAI GPT-4 integration with function calling
- 🛠️ Built-in tools:
  - Web search (DuckDuckGo)
  - Calculator
  - Date/Time
  - HTTP fetch
- 💾 Persistent conversation memory (SQLite)
- 🔌 RESTful API
- 📝 Session management

## Prerequisites

- Go 1.22 or higher
- OpenAI API key

## Installation

1. Clone the repository:
```bash
git clone https://github.com/HolyExcalibor/KAi-agent.git
cd KAi-agent
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file:
```env
OPENAI_API_KEY=your_api_key_here
OPENAI_MODEL=gpt-4o
SERVER_PORT=8080
DB_PATH=./kai.db
```

## Usage

Run the server:
```bash
go run cmd/server/main.go
```

The API will be available at `http://localhost:8080`

## Project Structure

```
kai-agent/
├── cmd/
│   └── server/          # Application entry point
├── config/              # Configuration management
├── internal/
│   ├── agent/          # Core agent logic
│   ├── api/            # HTTP API handlers
│   ├── memory/         # Persistent storage
│   └── tools/          # Built-in tools
├── go.mod
└── README.md
```

## License

MIT
