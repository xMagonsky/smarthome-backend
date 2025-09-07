# SmartHome Backend

## Overview
This repository contains the backend implementation for a SmartHome system, designed to manage and control smart home devices. The backend is built using GO and provides APIs for device control, user management, and system monitoring.

## Features
- **Device Management**: Control and monitor smart home devices (e.g., lights, sensors).
- **User Authentication**: User login and access control.
- **API Endpoints**: RESTful APIs for interacting with the frontend or external applications.
- **Real-time Updates**: MQTT for real-time device status updates.
- **Smart Automatization**: Creation of complex rules and actions that run when certain conditions are met.

## Tech Stack
- **Language**: GO
- **Services**: PostgreSQL, Redis, EMQX
- **Other tools**: Docker

## Installation
1. **Clone the repository**:
   ```bash
   git clone https://github.com/xMagonsky/smarthome-backend.git
   cd smarthome-backend
   ```

2. **Set up environment variables**:
   Create a `.env` file in the root directory based on *.example.env*:
   ```bash
   [e.g., nano .env]
   ```

3. **Build server with docker compose**:
   ```bash
   docker compose build
   ```

4. **Start the server**:
   ```bash
   docker compose up -d
   ```

## API Endpoints
[Swagger-type documentation](https://github.com/xMagonsky/smarthome-backend/blob/main/swagger.yaml)
