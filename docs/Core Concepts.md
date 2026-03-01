---
title: Reddit Clone - Core Concepts
tags: [type/concept, proj/reddit-clone, status/active]
summary: Core features and domain model of the Reddit clone project
---

## 1. Users and Authentication

- Registration requires only a **username + password** (email optional, for recovery only)
- Users are identified by **pseudonymous usernames** — no real name, no phone number
- Each user has a **profile** showing: posts, comments, karma score, cake day (join date)
- **Karma** is a reputation score — sum of upvotes received on posts and comments
- Sessions are managed via tokens (JWT or session-based)

## 2. Communities (Subreddits)

- Communities are prefixed with `r/` (e.g., `r/programming`)
- Any authenticated user can **create** a community
- Community attributes:
  - **Name** — unique, immutable once created
  - **Description / About** — markdown supported
  - **Rules** — community-specific guidelines
  - **Banner and Icon** images
  - **Member count**
  - **Visibility** — public, restricted, or private
- Communities are the primary unit of content organization

## 3. User-Community Relationship

- Users **join/leave** communities (subscribe/unsubscribe)
- Roles within a community:
  - **Member** — can post, comment, and vote
  - **Moderator** — can remove posts/comments, ban users, edit settings
  - **Creator/Owner** — the user who created the subreddit (also a moderator)
- A user's **home feed** aggregates posts from all joined communities

## 4. Posts

- Every post belongs to a **single community**
- Post types:
  - **Text post** — title + body (markdown)
  - **Link post** — title + URL
  - **Image/Video post** — title + media upload
- Post attributes:
  - Title (required)
  - Author (username)
  - Community
  - Timestamp (created, edited)
  - Vote score (upvotes minus downvotes)
  - Comment count
  - Flair/tags (optional, community-specific)

## 5. Comments

- Comments are **threaded/nested** (tree structure, not flat)
- Each comment can be **replied to**, creating nested threads
- Comments have their own **upvotes/downvotes** and score
- Comments are tied to a specific post
- Deleted comments show `[deleted]` but the thread structure is preserved

## 6. Voting System

- Every post and comment can be **upvoted** or **downvoted**
- One vote per user per item (can change or remove vote)
- Net score = upvotes - downvotes
- Voting drives **content ranking** (what appears at the top)
- A user's karma = total upvotes received across all their posts and comments

## 7. Sorting and Feed Algorithms

- Posts can be sorted by:
  - **Hot** — recent + high engagement (default)
  - **New** — reverse chronological
  - **Top** — highest net score (filterable: today, week, month, year, all time)
  - **Rising** — gaining upvotes quickly
- Comments can be sorted by: Best, Top, New, Controversial
- Home feed pulls from all subscribed communities, sorted by the selected algorithm

## 8. Search

- Search across all communities or within a specific community
- Searchable fields: post title, post body, community name
- Autocomplete suggestions for community names

## 9. Save and Share

- Users can **save** posts and comments for later viewing
- Posts and comments have shareable **permalink URLs**

## 10. Moderation

- Moderators can:
  - Remove posts and comments
  - Ban/mute users from their community
  - Pin/sticky posts (up to 2 per community)
  - Set and enforce community rules
  - Approve or reject posts (if community requires approval)
  - Assign flair to posts

## 11. Feature Priority Matrix

| Priority | Feature                              | Complexity |
| -------- | ------------------------------------ | ---------- |
| P0       | User auth (register/login/logout)    | Medium     |
| P0       | Create and join communities          | Medium     |
| P0       | Create posts (text at minimum)       | Medium     |
| P0       | Nested comments                      | High       |
| P0       | Upvote/Downvote on posts and comments | Medium     |
| P1       | Home feed (aggregated from joined)   | Medium     |
| P1       | Post sorting (hot/new/top)           | Medium     |
| P1       | User profiles with karma             | Low        |
| P2       | Image/video posts                    | Medium     |
| P2       | Search                               | Medium     |
| P2       | Moderation tools                     | High       |
| P3       | Save posts                           | Low        |
| P3       | Flairs/tags                          | Low        |
