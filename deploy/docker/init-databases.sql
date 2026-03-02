-- Creates all databases needed for Phase 2 services.
-- This script is mounted into PostgreSQL's docker-entrypoint-initdb.d/
-- and runs automatically on first container startup.

CREATE DATABASE auth;
CREATE DATABASE user_profiles;
CREATE DATABASE community;

GRANT ALL PRIVILEGES ON DATABASE auth TO redyx;
GRANT ALL PRIVILEGES ON DATABASE user_profiles TO redyx;
GRANT ALL PRIVILEGES ON DATABASE community TO redyx;
