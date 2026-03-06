-- Creates all databases needed for services.
-- This script is mounted into PostgreSQL's docker-entrypoint-initdb.d/
-- and runs automatically on first container startup.

-- Phase 2 databases
CREATE DATABASE auth;
CREATE DATABASE user_profiles;
CREATE DATABASE community;

GRANT ALL PRIVILEGES ON DATABASE auth TO redyx;
GRANT ALL PRIVILEGES ON DATABASE user_profiles TO redyx;
GRANT ALL PRIVILEGES ON DATABASE community TO redyx;

-- Phase 3 databases: post shards
CREATE DATABASE posts_shard_0;
CREATE DATABASE posts_shard_1;

GRANT ALL PRIVILEGES ON DATABASE posts_shard_0 TO redyx;
GRANT ALL PRIVILEGES ON DATABASE posts_shard_1 TO redyx;

-- Phase 5 databases
CREATE DATABASE notifications;
CREATE DATABASE media;

GRANT ALL PRIVILEGES ON DATABASE notifications TO redyx;
GRANT ALL PRIVILEGES ON DATABASE media TO redyx;

-- Phase 6 databases
CREATE DATABASE moderation;

GRANT ALL PRIVILEGES ON DATABASE moderation TO redyx;
