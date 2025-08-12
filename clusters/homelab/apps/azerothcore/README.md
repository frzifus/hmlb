# AzerothCore World of Warcraft Server

A complete World of Warcraft private server deployment based on AzerothCore, featuring separate auth and world servers with MySQL database backend and web-based administration tools.

## Components

### Server Infrastructure
- **[auth-server.yaml](./auth-server.yaml)** - Authentication server handling login and character selection
- **[world-server.yaml](./world-server.yaml)** - World server managing game world and player interactions
- **[database.yaml](./database.yaml)** - MySQL database cluster for game data storage
- **[phpmyadmin.yaml](./phpmyadmin.yaml)** - Web-based database administration interface

### Server Configuration
- **[auth-config.yaml](./auth-config.yaml)** - Authentication server configuration
- **[world-config.yaml](./world-config.yaml)** - World server game mechanics configuration
- **[ahbot-config.yaml](./ahbot-config.yaml)** - Auction house bot configuration for automated economy
- **[playerbots-config.yaml](./playerbots-config.yaml)** - AI player bot configuration

### Initialization
- **[init-job.yaml](./init-job.yaml)** - Database initialization and schema setup job
- **[namespace.yaml](./namespace.yaml)** - Dedicated namespace for the game server

## Features

### Complete WoW Server Experience
- **Multi-Realm Support**: Supports multiple game realms with load balancing
- **Custom Content**: Ability to add custom quests, items, and game mechanics
- **Player Management**: Web-based player administration and statistics
- **Economy Simulation**: Automated auction house bot for realistic economy

### Database Architecture
- **Replicated MySQL**: High-availability MySQL setup with read replicas
- **World Database**: Complete game world data including NPCs, quests, and items
- **Character Database**: Player character data, inventories, and progression
- **Auth Database**: Account management and realm authentication

### AI and Automation
- **Player Bots**: AI-controlled characters to populate the world
- **Auction House Bot**: Automated economy management and item pricing
- **Dynamic Content**: Server-side scripts for custom events and mechanics
- **Anti-Cheat**: Built-in protection against common exploits

## Server Configuration

### Authentication Server
The auth server handles:
- **Player Authentication**: Account login and validation
- **Realm List**: Available game worlds and their status
- **Character Selection**: Character listing and creation interface
- **Session Management**: Login sessions and timeout handling

### World Server
The world server manages:
- **Game World**: All in-game mechanics, NPCs, and player interactions
- **Chat Systems**: Global and local chat functionality
- **Guild Management**: Player guilds and social features
- **Instance Management**: Dungeons and raid instance creation

### Database Configuration
MySQL database includes:
- **Performance Tuning**: Optimized for game server workloads
- **Backup Strategy**: Automated backups with point-in-time recovery
- **Replication**: Master-slave setup for read scaling
- **Monitoring**: Database performance and health monitoring

## Storage and Performance

### Persistent Storage
- **Database Storage**: High-performance storage for MySQL data
- **Configuration Storage**: Persistent storage for server configurations
- **Log Storage**: Log aggregation and retention for troubleshooting
- **Asset Storage**: Game assets and custom content storage

### Performance Optimization
- **Resource Allocation**: CPU and memory tuning for server components
- **Network Optimization**: Low-latency networking for real-time gameplay
- **Database Indexing**: Optimized database queries for game operations
- **Caching Strategy**: In-memory caching for frequently accessed data

## Networking and Access

### External Connectivity
- **Game Client Access**: Direct connections for WoW clients
- **Web Administration**: Secure web interface access
- **Database Access**: Administrative database connectivity
- **Monitoring Interfaces**: Observability dashboard access

### Port Configuration
- **Auth Server**: Standard authentication server ports
- **World Server**: Game world communication ports
- **Database**: MySQL connection ports with security restrictions
- **Web Interface**: HTTP/HTTPS ports for administration

## Bot and AI Configuration

### Player Bots
Configurable AI players that:
- **Populate World**: Provide players for dungeons and world interaction
- **Follow Quests**: Complete quests and participate in world events
- **Social Interaction**: Respond to player communication
- **Combat Assistance**: Participate in player vs. environment content

### Auction House Bot
Automated economy management:
- **Price Management**: Dynamic pricing based on supply and demand
- **Item Circulation**: Ensures availability of essential items
- **Economy Balance**: Prevents inflation and market manipulation
- **Custom Policies**: Configurable rules for different item categories

## Monitoring and Administration

### Server Monitoring
- **Player Statistics**: Online players, server load, and performance metrics
- **Database Monitoring**: Query performance, connection counts, and storage usage
- **Resource Usage**: CPU, memory, and network utilization tracking
- **Error Tracking**: Server errors, crashes, and performance issues

### Administrative Tools
- **PhpMyAdmin**: Direct database access and management
- **Server Commands**: In-game administrative commands and tools
- **Player Management**: Account creation, banning, and character modification
- **Content Management**: Quest editing, item creation, and world modification

### Backup and Recovery
- **Database Backups**: Automated daily backups with retention policies
- **Configuration Backups**: Server configuration version control
- **Character Recovery**: Point-in-time character restoration
- **Disaster Recovery**: Complete server restoration procedures

## Security and Access Control

### Server Security
- **DDoS Protection**: Network-level protection against attacks
- **Anti-Cheat Systems**: Detection and prevention of game exploits
- **Access Control**: IP-based restrictions and authentication requirements
- **Encrypted Communication**: Secure client-server communication

### Database Security
- **Access Restrictions**: Limited database access with role-based permissions
- **Credential Management**: Encrypted storage of database credentials
- **Audit Logging**: Database access and modification logging
- **Backup Encryption**: Encrypted backup storage for data protection

## Customization and Modding

### Custom Content
The server supports:
- **Custom Quests**: Player-created quest lines and storylines
- **Custom Items**: Unique items with custom properties and effects
- **Custom NPCs**: Scripted NPCs with complex behaviors
- **World Modifications**: Terrain and zone modifications

### Scripting System
- **C++ Scripts**: Native server-side scripting for complex mechanics
- **Database Scripts**: SQL-based content creation and modification
- **Event System**: Trigger-based events and automated actions
- **Addon Support**: Client-side addon compatibility and support

## Maintenance and Updates

### Regular Maintenance
- **Database Optimization**: Regular index rebuilding and query optimization
- **Log Rotation**: Automated log cleanup and archival
- **Security Updates**: Regular server security patches and updates
- **Performance Tuning**: Ongoing performance monitoring and optimization

### Content Updates
- **Patch Deployment**: Controlled deployment of game content updates
- **Database Migrations**: Schema updates and data migration procedures
- **Configuration Updates**: Server setting updates and testing
- **Rollback Procedures**: Quick rollback for problematic updates

## Troubleshooting

### Common Issues
- **Connection Problems**: Client connectivity and authentication issues
- **Performance Issues**: Server lag, high latency, and resource bottlenecks
- **Database Problems**: Query performance, corruption, and connectivity issues
- **Bot Malfunctions**: AI behavior problems and configuration errors

### Debugging Tools
- **Server Logs**: Comprehensive logging for all server components
- **Database Analysis**: Query performance analysis and optimization tools
- **Network Diagnostics**: Connection testing and latency measurement
- **Performance Profiling**: Resource usage analysis and bottleneck identification