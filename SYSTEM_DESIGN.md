# Inventario System Design Document

## Overview

Inventario is a comprehensive personal inventory management system designed to help users organize, track, and manage their belongings. The system consists of a Go-based backend API, a Vue.js frontend.

## System Architecture

### High-Level Components

The Inventario system is composed of three main components:

1. **Frontend Application** - Vue.js-based web interface
2. **Backend API Server** - Go-based REST API with multiple database support

### Technology Stack

- **Backend**: Go 1.24+, Chi router, Swagger documentation
- **Frontend**: Vue.js 3, TypeScript, PrimeVue UI components, Pinia state management
- **Databases**: PostgreSQL (recommended), MySQL/MariaDB, BoltDB (embedded), In-memory
- **File Storage**: Go Cloud Development Kit (supports local, S3, Azure, GCS)

## Core Domain Concepts

### Data Model Hierarchy

The system follows a hierarchical organization structure:

```
Locations (Top-level containers)
├── Areas (Sub-containers within locations)
    └── Commodities (Individual items)
        ├── Images (Visual documentation)
        ├── Invoices (Purchase documentation)
        └── Manuals (Product documentation)
```

### Entity Definitions

#### Location
- **Purpose**: Top-level organizational unit (e.g., "Home", "Office", "Storage Unit")
- **Attributes**: ID, Name, Address
- **Relationships**: Contains multiple Areas

#### Area
- **Purpose**: Sub-division within a location (e.g., "Living Room", "Kitchen", "Basement")
- **Attributes**: ID, Name, Location ID
- **Relationships**: Belongs to one Location, contains multiple Commodities

#### Commodity
- **Purpose**: Individual trackable item with comprehensive metadata
- **Key Attributes**:
  - Basic Info: Name, Short Name, Type, Count
  - Financial: Original Price, Current Price, Currency handling
  - Identification: Serial Numbers, Part Numbers
  - Status: Draft, Active, Sold, Lost, Disposed, Written Off
  - Dates: Purchase Date, Registration Date, Last Modified
  - Organization: Tags, Comments, URLs
- **Relationships**: Belongs to one Area, has multiple Files (Images, Invoices, Manuals)

#### File Management
- **File Types**: Images, Invoices, Manuals
- **Metadata**: Original path, editable filename, extension, MIME type
- **Features**: In-app viewing (images with zoom, PDFs with navigation)

### Business Rules

#### Status Hierarchy
The system implements a visual status hierarchy for commodities:
1. **Draft** (highest priority) - Items being prepared
2. **Sold** - Grayscale with diagonal stamp
3. **Lost** - Yellow overlay
4. **Disposed** - Semi-transparent
5. **Written Off** - Faded appearance

#### Value Calculations
- Ignores draft items in calculations
- Defaults to USD if no main currency is set
- Prioritizes current price over original price
- Handles currency conversion for original prices

## Frontend Architecture

### Component Structure

The Vue.js frontend follows a hierarchical navigation pattern:

```
Location List → Location Detail (with Areas) → Area Detail (with Commodities) → Commodity Detail
```

### Key Features

#### Navigation & UX
- Hierarchical flow with consistent back navigation
- Context preservation when navigating back
- Highlighting of previously edited items
- Entirely clickable cards with separate edit functionality

#### Form Management
- Reusable form components extracted from Create forms
- Automatic error scrolling and validation
- Consistent PrimeVue component usage
- Grouped area selection by parent location

#### State Management
- Pinia stores for global state
- Settings store for main currency and preferences
- Reactive updates across components

#### File Handling
- Drag-and-drop file uploads
- In-app image viewer with zoom capabilities
- PDF.js integration for document viewing
- File renaming capabilities in UI

## Backend Architecture

### API Design

The backend follows RESTful principles with JSON:API specification:

- **Locations API**: CRUD operations for location management
- **Areas API**: Area management within locations
- **Commodities API**: Comprehensive item management with file attachments
- **Settings API**: Global configuration management
- **Uploads API**: File upload and management
- **Values API**: Aggregated value calculations

### Database Support

#### Multi-Database Architecture
The system supports multiple database backends through a registry pattern:

- **PostgreSQL**: Full-featured production database with advanced features
- **MySQL/MariaDB**: Alternative SQL database with full feature support
- **BoltDB**: Embedded key-value store for single-user deployments
- **Memory**: In-memory storage for testing and development

#### Registry Pattern
Each database type implements a common interface:
```go
type Registry interface {
    LocationRegistry
    AreaRegistry
    CommodityRegistry
    SettingsRegistry
}
```

### File Storage

Uses Go Cloud Development Kit for flexible file storage:
- Local filesystem (development)
- Amazon S3 (production)
- Google Cloud Storage
- Azure Blob Storage
- In-memory (testing)

## Security & Data Integrity

### Validation
- Context-aware validation using `jellydator/validation`
- Consistent validation patterns across all entities
- Client-side and server-side validation

### Transaction Safety
- All database operations wrapped in transactions
- Atomic operations for complex updates
- Rollback capabilities for failed operations

### File Security
- MIME type validation
- File extension verification
- Secure file serving with proper headers

## Development & Deployment

### Build System
- Makefile-based build automation
- Frontend and backend compilation
- Asset bundling and optimization

### Testing Strategy
- Unit tests with quicktest framework
- Integration tests for database operations
- End-to-end tests with Playwright
- Table-driven tests for comprehensive coverage

### Deployment Options
- Single binary deployment
- Docker containerization
- Database migration automation
- Dry-run mode for safe operations

## Future Considerations

### Scalability
- Horizontal scaling through stateless API design
- Database connection pooling
- File storage CDN integration

### Features
- Multi-user support with authentication
- Advanced reporting and analytics
- Mobile application development
- API rate limiting and caching

### Monitoring
- Structured logging
- Performance metrics
- Health check endpoints
- Error tracking and alerting
