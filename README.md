# Duitno

Payment gateway for Indonesian streamers to receive donations.

## Overview

Duitno is a payment gateway solution designed specifically for Indonesian content creators and streamers. It enables viewers to send donations to their favorite streamers through various payment methods commonly used in Indonesia.

## Features

- OAuth authentication for streamers
- Multiple payment method integrations
- Real-time donation notifications
- Dashboard for streamers to manage donations

## Tech Stack

- **Backend**: Go with Fiber framework
- **Database**: SQL (with migrations)
- **Authentication**: OAuth 2.0

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL (or compatible database)
- OAuth credentials from payment providers

### Installation

1. Clone the repository
2. Copy `.env.example` to `.env` and configure
3. Run migrations: `make migrate`
4. Build and run: `make run`

### Configuration

Create a `.env` file based on `.env.example`:

```
PORT=8080
DATABASE_URL=postgres://user:pass@localhost:5432/duitno
OAUTH_CLIENT_ID=your_client_id
OAUTH_CLIENT_SECRET=your_client_secret
```

## API Endpoints

- `GET /health` - Health check
- `GET /health/db` - Database health check
- `GET /auth/:provider` - Initiate OAuth flow
- `GET /auth/:provider/callback` - OAuth callback

## License

MIT
