# Enshrouded Dedicated Server

A dedicated Enshrouded game server deployment providing persistent multiplayer survival gameplay with automated backup and monitoring capabilities.

## Components

- **[server.yaml](./server.yaml)** - Main Enshrouded dedicated server deployment
- **[namespace.yaml](./namespace.yaml)** - Dedicated namespace for the game server

## Features

### Persistent Multiplayer World
- **Dedicated Server**: Always-online server for consistent multiplayer experience
- **World Persistence**: Player builds and progress saved permanently
- **Session Management**: Handles player connections and world state synchronization
- **Performance Optimization**: Tuned for stable multiplayer performance

### Survival Gameplay Support
The server provides infrastructure for:
- **Cooperative Building**: Shared building and crafting experiences
- **Resource Management**: Persistent resource collection and storage
- **Progress Tracking**: Individual and group progression persistence
- **World Events**: Server-managed events and encounters

## Server Configuration

### Game Settings
- **World Size**: Configurable world generation parameters
- **Player Limit**: Maximum concurrent player connections
- **Difficulty Settings**: Survival mechanics and challenge configuration
- **PvP Settings**: Player vs. player combat rules and restrictions

### Performance Tuning
- **Resource Allocation**: CPU and memory optimization for smooth gameplay
- **Network Configuration**: Low-latency networking for responsive multiplayer
- **Save Intervals**: Automated world saving to prevent data loss
- **Cleanup Procedures**: Periodic cleanup of temporary game data

## Storage and Persistence

### World Data Storage
- **Persistent Volumes**: Dedicated storage for world saves and player data
- **Backup Strategy**: Automated backups of world state and player progress
- **Storage Performance**: Fast storage for real-time world updates
- **Data Integrity**: Checksums and validation for save file protection

### Configuration Management
- **Server Settings**: Persistent storage of server configuration
- **Mod Support**: Storage for server-side modifications and custom content
- **Log Retention**: Game event and error log storage
- **Update Management**: Version control for server updates

## Networking and Access

### Player Connectivity
- **Direct Connection**: Direct IP connection for game clients
- **Port Configuration**: Standard Enshrouded server ports
- **Connection Limits**: Configurable player connection limits
- **Session Handling**: Graceful handling of player joins and leaves

### External Access
- **Tailscale Integration**: Secure access for remote players
- **Firewall Configuration**: Controlled access through network policies
- **Load Balancing**: Connection distribution for server stability
- **DDoS Protection**: Network-level protection against attacks

## Monitoring and Administration

### Server Health Monitoring
- **Resource Usage**: CPU, memory, and disk utilization tracking
- **Player Metrics**: Online player count and connection statistics
- **Performance Metrics**: Server tick rate, lag, and response times
- **Error Tracking**: Server errors, crashes, and recovery procedures

### Administrative Tools
- **Server Console**: Direct server console access for administration
- **Player Management**: Player kick, ban, and privilege management
- **World Management**: World backup, restore, and reset capabilities
- **Configuration Updates**: Hot-reload of server settings where possible

### Logging and Diagnostics
- **Game Event Logs**: Player actions, world events, and server activities
- **Error Logs**: Server errors, warnings, and diagnostic information
- **Performance Logs**: Resource usage and performance metrics
- **Audit Logs**: Administrative actions and configuration changes

## Backup and Recovery

### Automated Backups
- **World Saves**: Regular automated backups of world state
- **Player Data**: Individual player progress and inventory backups
- **Configuration Backups**: Server settings and mod configuration backups
- **Incremental Backups**: Efficient incremental backup strategy

### Recovery Procedures
- **World Restoration**: Point-in-time world state recovery
- **Player Recovery**: Individual player data restoration
- **Rollback Procedures**: Server rollback for corrupted saves
- **Disaster Recovery**: Complete server restoration from backups

## Security and Access Control

### Server Security
- **Authentication**: Player authentication and validation
- **Anti-Cheat**: Basic cheat detection and prevention measures
- **Access Control**: IP-based access restrictions and whitelisting
- **Secure Communication**: Encrypted client-server communication

### Administrative Security
- **Console Access**: Restricted access to server console and commands
- **Configuration Security**: Protected server configuration files
- **Backup Security**: Encrypted backup storage and access control
- **Audit Trail**: Complete administrative action logging

## Performance Optimization

### Resource Management
- **CPU Allocation**: Dedicated CPU resources for server stability
- **Memory Management**: Optimized memory usage for large worlds
- **Storage Performance**: Fast I/O for world loading and saving
- **Network Optimization**: Low-latency networking configuration

### Scaling Considerations
- **Player Capacity**: Optimal player count for server hardware
- **World Size Limits**: Recommended world size for performance
- **Mod Impact**: Performance impact assessment for server modifications
- **Hardware Requirements**: Minimum and recommended server specifications

## Troubleshooting

### Common Issues
- **Connection Problems**: Player connectivity and timeout issues
- **Performance Issues**: Server lag, stuttering, and resource bottlenecks
- **Save Corruption**: World save file corruption and recovery
- **Mod Conflicts**: Server modification conflicts and compatibility

### Debugging Tools
- **Server Logs**: Comprehensive logging for troubleshooting
- **Performance Monitoring**: Real-time performance metrics and analysis
- **Network Diagnostics**: Connection testing and latency measurement
- **Resource Analysis**: CPU, memory, and disk usage analysis

### Maintenance Procedures
- **Regular Restarts**: Scheduled server restarts for stability
- **Update Procedures**: Safe server update and rollback procedures
- **Database Maintenance**: World save optimization and cleanup
- **Performance Tuning**: Ongoing optimization and configuration adjustment

## Game Server Management

### Server Lifecycle
- **Startup Procedures**: Automated server startup and initialization
- **Graceful Shutdown**: Safe server shutdown with player notification
- **Update Management**: Controlled server updates with minimal downtime
- **Maintenance Windows**: Scheduled maintenance and backup procedures

### Player Experience
- **Connection Stability**: Stable connections with automatic reconnection
- **World Persistence**: Reliable world state saving and loading
- **Performance Consistency**: Stable frame rates and low latency
- **Community Features**: Support for player communication and coordination