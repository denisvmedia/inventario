#!/bin/bash

# Docker Test Script for Inventario
# This script tests the Docker setup

set -e

echo "🐳 Testing Inventario Docker Setup"
echo "=================================="

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed or not in PATH"
    exit 1
fi

# Check if Docker Compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed or not in PATH"
    exit 1
fi

echo "✅ Docker and Docker Compose are available"

# Check if .env file exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from .env.example"
    cp .env.example .env
fi

echo "✅ Environment file is ready"

# Build the Docker image
echo "🔨 Building Docker image..."
docker build -t inventario:test .

echo "✅ Docker image built successfully"

# Start services
echo "🚀 Starting services..."
docker-compose up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to be healthy..."
sleep 30

# Check if services are running
if ! docker-compose ps | grep -q "Up"; then
    echo "❌ Services are not running properly"
    docker-compose logs
    exit 1
fi

echo "✅ Services are running"

# Test API endpoint
echo "🧪 Testing API endpoint..."
if curl -f http://localhost:3333/api/v1/settings > /dev/null 2>&1; then
    echo "✅ API is responding"
else
    echo "❌ API is not responding"
    docker-compose logs inventario
    exit 1
fi

# Seed the database
echo "🌱 Seeding database..."
if curl -X POST http://localhost:3333/api/v1/seed > /dev/null 2>&1; then
    echo "✅ Database seeded successfully"
else
    echo "⚠️  Database seeding failed (this might be expected if already seeded)"
fi

echo ""
echo "🎉 Docker setup test completed successfully!"
echo ""
echo "📋 Next steps:"
echo "   • Access the application at: http://localhost:3333"
echo "   • Stop services with: docker-compose down"
echo "   • View logs with: docker-compose logs -f"
echo ""
