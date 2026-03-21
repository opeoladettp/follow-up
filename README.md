# AI-Native Newsroom Backend

High-performance Go backend for Followupmedium.com's AI-Native Newsroom, implementing the "Follow-up" model for tracking story developments.

## Architecture Overview

The system implements a stateful follow-up logic that:
- Periodically fetches data from RSS feeds and X (Twitter) handles
- Uses Redis for SHA-256 hash comparison to detect content changes
- Triggers MongoDB updates when new developments are detected
- Employs a worker pool for concurrent, non-blocking operations

## Key Features

### 🔄 Stateful Follow-up Engine
- **Diff Detection**: SHA-256 hash comparison in Redis
- **Story Lifecycle**: MongoDB documents tracking story evolution
- **Worker Pool**: Concurrent content fetching without blocking
- **Rate Limiting**: Intelligent API rate limiting for X scraping

### 🤖 AI Integration
- **Gemini 3 Pro**: Content analysis and script enhancement
- **Veo 3.1**: 4K video generation
- **Nano Banana**: Avatar consistency across videos
- **MCP Protocol**: Bridge between Go backend and AI Host

### 📊 Production-Grade Features
- **Graceful Shutdown**: Proper MongoDB/Redis connection handling
- **Telemetry**: Real-time metrics for Looker Studio dashboard
- **Type Safety**: Comprehensive error handling throughout
- **Modular Design**: Easy to swap X scraper for official API

## Quick Start

### Prerequisites
- Go 1.21+
- MongoDB
- Redis
- API keys for Gemini, Veo, and Nano Banana

### Installation

1. Clone and setup:
```bash
git clone <repository>
cd followupmedium-newsroom
cp .env.example .env
# Edit .env with your API keys and database URLs
```

2. Install dependencies:
```bash
go mod tidy
```

3. Run the application:
```bash
go run main.go
```

The server will start on port 8080 (HTTP API) and 8081 (MCP server).

## API Endpoints

### HTTP API (Port 8080)
- `GET /health` - Health check
- `GET /api/v1/stories` - List active stories
- `POST /api/v1/stories` - Create new story
- `GET /api/v1/stories/:id/context` - Get story timeline
- `POST /api/v1/kpi/update` - Update KPI dashboard

### MCP Server (Port 8081)
- `POST /mcp` - MCP protocol endpoint
- `GET /mcp/tools` - List available MCP tools

## MCP Tools

The system exposes three MCP tools to the Gemini 3 Host:

### 1. get_story_context(story_id)
Retrieves the full timeline from MongoDB for a specific story.

### 2. trigger_production_pipeline(script_text)
Sends a script to Veo 3.1 API for 4K video generation and Nano Banana for avatar overlay.

### 3. update_kpi_dashboard()
Pushes real-time metrics to MongoDB for Looker Studio dashboard.

## Project Structure

```
├── main.go                 # Application entry point
├── internal/
│   ├── config/            # Configuration management
│   ├── database/          # MongoDB and Redis clients
│   ├── models/            # Data models and structures
│   ├── services/          # Business logic services
│   │   ├── story_service.go    # Story management
│   │   ├── diff_engine.go      # Content diffing logic
│   │   └── ai_service.go       # AI integration
│   ├── workers/           # Worker pool and content fetching
│   ├── mcp/              # Model Context Protocol server
│   └── api/              # HTTP API routes and middleware
├── go.mod
└── README.md
```

## Environment Variables

See `.env.example` for all required environment variables.

## Key Implementation Details

### Diff Engine
The diff engine compares new content against stored SHA-256 hashes in Redis:
- **Breaking News**: First content for a story or contains breaking keywords
- **Follow-up**: Subsequent developments in existing stories
- **Duplicate**: Already processed content (skipped)

### Worker Pool
Concurrent worker pool handles:
- RSS feed parsing
- Twitter content fetching (placeholder for official API)
- Content diffing and MongoDB updates
- Rate limiting to prevent API abuse

### Avatar Consistency
When triggering video production, the system ensures avatar consistency by:
- Referencing a static "Identity Image" URL
- Passing this to Nano Banana for consistent visual identity
- Maintaining the same AI correspondent across all videos

### Telemetry & KPIs
Every operation logs success/failure to MongoDB telemetry collection:
- API latency tracking
- Error rate monitoring
- Story processing metrics
- Ready for Looker Studio integration

## Development Notes

### Week 1 Focus Areas
1. **Diff Engine**: Logic to distinguish "Breaking" vs "Follow-up" events
2. **Avatar Consistency**: Static identity image reference for Nano Banana
3. **Dashboard Readiness**: Comprehensive telemetry logging

### Future Enhancements
- Replace X scraper placeholder with official Twitter API v2
- Add more sophisticated content analysis
- Implement video processing status tracking
- Add webhook support for real-time notifications

## Contributing

1. Ensure all code includes proper error handling
2. Add telemetry logging for new operations
3. Update tests for new functionality
4. Follow Go best practices and conventions

## License

[Your License Here]