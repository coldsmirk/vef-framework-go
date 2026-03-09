# Test Fixture Data Reference

This document describes the test fixture data used by the ORM test suite. All timestamps are within 2025; test-inserted data uses 2026+ timestamps and is filtered out by `fixtureScope`.

---

## Entity Relationship Diagram

```
User (test_user)
├── 1:N → Post (test_post)              via user_id
├── 1:N → Comment (test_comment)        via user_id
├── 1:N → UserFavorite (test_user_favorite) via user_id
│
Post (test_post)
├── N:1 → User (test_user)              via user_id
├── N:1 → Category (test_category)      via category_id
├── N:M → Tag (test_tag)                via PostTag (test_post_tag)
├── 1:N → Comment (test_comment)        via post_id
├── 1:N → UserFavorite (test_user_favorite) via post_id
│
Category (test_category)
├── self-ref → parent_id                (tree structure)
├── 1:N → Post (test_post)              via category_id
│
Comment (test_comment)
├── N:1 → Post (test_post)              via post_id
├── N:1 → User (test_user)              via user_id
├── self-ref → parent_id                (tree structure, max depth 4)
│
Tag (test_tag)
├── N:M → Post (test_post)              via PostTag (test_post_tag)
│
PostTag (test_post_tag)
├── N:1 → Post (test_post)              via post_id
├── N:1 → Tag (test_tag)                via tag_id
│
UserFavorite (test_user_favorite)        ★ composite PK (user_id, post_id)
├── N:1 → User (test_user)              via user_id
├── N:1 → Post (test_post)              via post_id
```

---

## Users (20 records)

| ID | Name | Email | Age | Active | Created |
|---|---|---|---|---|---|
| usr001 | Alice Johnson | alice@example.com | 30 | ✓ | 2025-01-15 |
| usr002 | Bob Smith | bob@example.com | 25 | ✓ | 2025-02-20 |
| usr003 | Charlie Brown | charlie@example.com | 35 | ✗ | 2025-03-10 |
| usr004 | Diana Lee | diana@example.com | 28 | ✓ | 2025-03-25 |
| usr005 | Edward Kim | edward@example.com | 42 | ✓ | 2025-04-08 |
| usr006 | Fiona Chen | fiona@example.com | 22 | ✗ | 2025-04-22 |
| usr007 | George Wang | george@example.com | 33 | ✓ | 2025-05-15 |
| usr008 | Hannah Park | hannah@example.com | 27 | ✓ | 2025-05-30 |
| usr009 | Ivan Petrov | ivan@example.com | 45 | ✗ | 2025-06-12 |
| usr010 | Julia Santos | julia@example.com | 29 | ✓ | 2025-06-28 |
| usr011 | Kevin Murphy | kevin@example.com | 38 | ✓ | 2025-07-14 |
| usr012 | Laura Garcia | laura@example.com | 24 | ✗ | 2025-07-30 |
| usr013 | Michael Zhang | michael@example.com | 31 | ✓ | 2025-08-10 |
| usr014 | Nina Kowalski | nina@example.com | 26 | ✓ | 2025-08-25 |
| usr015 | Oscar Rivera | oscar@example.com | 40 | ✗ | 2025-09-08 |
| usr016 | Patricia Ngo | patricia@example.com | 23 | ✓ | 2025-09-22 |
| usr017 | Quentin Dubois | quentin@example.com | 36 | ✓ | 2025-10-05 |
| usr018 | Rachel Adams | rachel@example.com | 34 | ✗ | 2025-10-20 |
| usr019 | Samuel Okafor | samuel@example.com | 39 | ✓ | 2025-11-10 |
| usr020 | Tina Yamamoto | tina@example.com | 21 | ✓ | 2025-11-28 |

### User Statistics (Quick Reference for Tests)

- **Total**: 20
- **Active**: 14 (usr001/002/004/005/007/008/010/011/013/014/016/017/019/020)
- **Inactive**: 6 (usr003/006/009/012/015/018)
- **All ages are unique** (21–45), every `created_at != updated_at`
- `age = 30` → 1 (Alice), `age = 25` → 1 (Bob), `age = 35` → 1 (Charlie)
- `age > 30` → **10** (31, 33, 34, 35, 36, 38, 39, 40, 42, 45)
- `age < 30` → **9** (21, 22, 23, 24, 25, 26, 27, 28, 29)
- `age >= 30` → **11** (30 + the 10 above)
- `age <= 30` → **10** (30 + the 9 below)
- `age = 30` → **1** (Alice only)
- `Between(25, 30)` inclusive → **6** (ages: 25, 26, 27, 28, 29, 30)
- `Contains("Alice")` → 1, `Contains("Bob")` → 1 (no name overlaps)
- `NotEquals("Alice Johnson")` → 19
- `created_by = "system"` → 20 (all users)

---

## Categories (15 records) — Tree Structure

```
Technology (cat001)
├── GoLang (cat004)
│   └── Go Web Frameworks (cat011)
├── AI & Machine Learning (cat005)
│   └── Deep Learning (cat012)
└── Cloud Computing (cat009)

Science (cat002)
├── Physics (cat013)
└── Biology (cat014)

Business (cat003)
├── Startups (cat008)
└── Marketing (cat015)

Health & Fitness (cat006)

Education (cat007)
└── Online Courses (cat010)
```

### Category Statistics

- **Total**: 15
- **Root nodes** (no parent): 5 — cat001, cat002, cat003, cat006, cat007
- **Leaf nodes** (no children): 8 — cat006, cat008, cat009, cat010, cat011, cat012, cat013, cat014, cat015
- **Max depth**: 3 (cat001 → cat004 → cat011, cat001 → cat005 → cat012)
- **Technology subtree** (cat001): 5 descendants (cat004, cat005, cat009, cat011, cat012)
- **Science subtree** (cat002): 2 descendants (cat013, cat014)
- **Business subtree** (cat003): 2 descendants (cat008, cat015)

---

## Posts (30 records)

| ID | User | Category | Status | Views | Created |
|---|---|---|---|---|---|
| post001 | usr001 | cat004 (GoLang) | published | 423 | 2025-01-20 |
| post002 | usr002 | cat001 (Technology) | published | 287 | 2025-02-05 |
| post003 | usr001 | cat005 (AI & ML) | draft | 145 | 2025-02-18 |
| post004 | usr003 | cat003 (Business) | published | 164 | 2025-03-08 |
| post005 | usr001 | cat004 (GoLang) | review | 198 | 2025-03-22 |
| post006 | usr002 | cat002 (Science) | published | 177 | 2025-04-02 |
| post007 | usr003 | cat001 (Technology) | published | 152 | 2025-04-15 |
| post008 | usr001 | cat008 (Startups) | published | 269 | 2025-04-28 |
| post009 | usr004 | cat001 (Technology) | published | 356 | 2025-05-10 |
| post010 | usr005 | cat009 (Cloud) | published | 412 | 2025-05-22 |
| post011 | usr004 | cat001 (Technology) | draft | 34 | 2025-06-03 |
| post012 | usr006 | cat010 (Online Courses) | review | 112 | 2025-06-18 |
| post013 | usr007 | cat009 (Cloud) | published | 389 | 2025-07-01 |
| post014 | usr008 | cat002 (Science) | published | 234 | 2025-07-16 |
| post015 | usr005 | cat001 (Technology) | draft | 28 | 2025-07-28 |
| post016 | usr009 | cat013 (Physics) | published | 467 | 2025-08-08 |
| post017 | usr010 | cat001 (Technology) | published | 301 | 2025-08-20 |
| post018 | usr011 | cat001 (Technology) | published | 445 | 2025-09-02 |
| post019 | usr013 | cat012 (Deep Learning) | published | 378 | 2025-09-15 |
| post020 | usr012 | cat007 (Education) | review | 89 | 2025-09-28 |
| post021 | usr014 | cat001 (Technology) | published | 256 | 2025-10-05 |
| post022 | usr015 | cat007 (Education) | published | 134 | 2025-10-18 |
| post023 | usr016 | cat001 (Technology) | draft | 67 | 2025-10-28 |
| post024 | usr017 | cat001 (Technology) | published | 189 | 2025-11-05 |
| post025 | usr018 | cat002 (Science) | review | 156 | 2025-11-15 |
| post026 | usr019 | cat001 (Technology) | published | 312 | 2025-11-22 |
| post027 | usr020 | cat001 (Technology) | published | 223 | 2025-11-30 |
| post028 | usr007 | cat011 (Go Web) | published | 345 | 2025-12-05 |
| post029 | usr008 | cat014 (Biology) | draft | 78 | 2025-12-10 |
| post030 | usr011 | cat015 (Marketing) | review | 198 | 2025-12-15 |

### Post Statistics

- **Total**: 30
- **By status**: published 18, draft 6, review 6
- **By user**: usr001 has 4 posts, usr002/003/004/005/007/008/011 have 2 each, rest have 1
- **By category**: cat001 (Technology) has 12 posts (most popular)
- **View count range**: 28–467

---

## Tags (12 records)

| ID | Name |
|---|---|
| tag001 | programming |
| tag002 | database |
| tag003 | go |
| tag004 | tutorial |
| tag005 | advanced |
| tag006 | machine-learning |
| tag007 | cloud |
| tag008 | security |
| tag009 | mobile |
| tag010 | data-science |
| tag011 | design |
| tag012 | devops |

---

## PostTags (55 records)

Each post has 1–3 tags. Every tag is used at least twice.

---

## Comments (25 records) — Tree Structure

Comments are distributed across 5 posts with varying nesting depth.

### Comment Tree Visualization

```
Post "Introduction to Go" (post001) — 6 comments, max depth 2:
├── cmt001 (Bob, L0, 12♥) "Great introduction!"
│   ├── cmt002 (Alice, L1, 5♥) "Thanks Bob!"
│   │   └── cmt003 (Charlie, L2, 3♥) "I learned a lot too"
│   └── cmt004 (Diana, L1, 8♥) "Very helpful"
├── cmt005 (Edward, L0, 15♥) "Need more examples"
│   └── cmt006 (Alice, L1, 7♥) "Will add more soon"

Post "Database Design Basics" (post002) — 8 comments, max depth 4:
├── cmt007 (Alice, L0, 20♥) "Normalization is key"
│   ├── cmt008 (Charlie, L1, 14♥) "What about denormalization?"
│   │   ├── cmt009 (Alice, L2, 9♥) "Good point, it depends"
│   │   │   └── cmt010 (Bob, L3, 11♥) "Performance vs consistency"
│   │   │       └── cmt011 (Edward, L4, 18♥) "Always benchmark first"
│   │   └── cmt012 (Diana, L2, 6♥) "NoSQL handles this differently"
│   └── cmt013 (Fiona, L1, 4♥) "Great discussion"
├── cmt014 (George, L0, 10♥) "Foreign keys are essential"

Post "Cloud Security Best Practices" (post010) — 4 comments, max depth 1:
├── cmt015 (George, L0, 9♥)
│   └── cmt016 (Edward, L1, 13♥)
├── cmt017 (Kevin, L0, 7♥)
│   └── cmt018 (Samuel, L1, 5♥)

Post "Kubernetes in Production" (post013) — 3 comments, max depth 1:
├── cmt019 (Edward, L0, 16♥)
│   └── cmt020 (George, L1, 11♥)
├── cmt021 (Michael, L0, 8♥)

Post "Neural Network Architectures" (post019) — 4 comments, max depth 1:
├── cmt022 (Hannah, L0, 22♥)
│   └── cmt023 (Michael, L1, 17♥)
├── cmt024 (Ivan, L0, 10♥)
│   └── cmt025 (Michael, L1, 14♥)
```

### Comment Statistics

- **Total**: 25
- **Root comments** (no parent): 9 — cmt001, cmt005, cmt007, cmt014, cmt015, cmt017, cmt019, cmt021, cmt022, cmt024 — actually **10**
- **Max depth**: 4 (cmt007 → cmt008 → cmt009 → cmt010 → cmt011)
- **Deepest chain** (post002): 5 nodes spanning levels 0–4
- **Posts with comments**: post001(6), post002(8), post010(4), post013(3), post019(4)
- **Most active commenter**: usr001 (Alice) with 5 comments
- **Likes range**: 3–22

---

## UserFavorites (12 records) — Composite Primary Key

This table uses a composite primary key `(user_id, post_id)` for testing composite PK condition methods.

| UserID | PostID | Created |
|---|---|---|
| usr001 | post001 | 2025-06-01 |
| usr001 | post002 | 2025-06-01 |
| usr001 | post003 | 2025-06-01 |
| usr002 | post001 | 2025-06-02 |
| usr002 | post004 | 2025-06-02 |
| usr002 | post005 | 2025-06-02 |
| usr003 | post002 | 2025-06-03 |
| usr003 | post006 | 2025-06-03 |
| usr004 | post003 | 2025-06-04 |
| usr004 | post007 | 2025-06-04 |
| usr005 | post001 | 2025-06-05 |
| usr005 | post008 | 2025-06-05 |

### UserFavorite Statistics

- **Total**: 12
- **Users with favorites**: 5 (usr001–usr005)
- **By user**: usr001 has 3, usr002 has 3, usr003 has 2, usr004 has 2, usr005 has 2
- **Most favorited post**: post001 (favorited by usr001, usr002, usr005)
