# Cortex Usage Examples

Real-world scenarios demonstrating how Cortex solves common file organization challenges.

## Scenario 1: Multi-Client Consulting Work

### Problem
You're a consultant working on 3 client projects simultaneously. Files are scattered across different folders, and you need to quickly switch contexts.

### Traditional Approach (Folder-Based)
```
workspace/
├── client-acme/
│   ├── code/
│   ├── docs/
│   └── meetings/
├── client-beta/
│   ├── code/
│   └── contracts/
└── client-gamma/
    └── research/
```

**Limitations**:
- A shared library file can only live in one place
- Cross-client learnings are hard to track
- No way to mark urgent items across clients

### Cortex Approach

#### Step 1: Tag files by client context
```
Open: src/shared/utils.ts
Command: "Cortex: Assign context to current file"
Enter: "client-acme"

Open: src/shared/utils.ts (again)
Command: "Cortex: Assign context to current file"
Enter: "client-beta"
```

The same file now appears under **both** clients!

#### Step 2: Tag urgent items
```
Open: docs/acme-deadline.md
Command: "Cortex: Add tag to current file"
Enter: "urgent"

Open: contracts/beta-amendment.pdf
Command: "Cortex: Add tag to current file"
Enter: "urgent"
```

#### Result: Three Views

**By Context**:
```
client-acme (12 files)
├── src/shared/utils.ts
├── docs/acme-deadline.md
└── contracts/acme-master.pdf

client-beta (8 files)
├── src/shared/utils.ts
├── contracts/beta-amendment.pdf
└── docs/beta-proposal.md

client-gamma (5 files)
└── research/market-analysis.xlsx
```

**By Tag**:
```
urgent (2 files)
├── docs/acme-deadline.md
└── contracts/beta-amendment.pdf
```

**By Type**:
```
typescript (15 files)
├── src/shared/utils.ts
└── ...

pdf (8 files)
├── contracts/acme-master.pdf
├── contracts/beta-amendment.pdf
└── ...
```

### Benefit
- **No duplication**: `utils.ts` exists once, appears in multiple contexts
- **Cross-cutting views**: See all urgent items regardless of client
- **Quick switching**: Click context to see only relevant files

---

## Scenario 2: Open Source Project with Bug Fixes

### Problem
You're maintaining an open-source library. Multiple issues are in progress, each touching files across the codebase.

### Traditional Approach
- Create branches for each issue
- Keep mental map of which files relate to which issues
- Use TODO comments (pollutes code)

### Cortex Approach

#### Tag files by issue number
```
Open: src/auth/login.ts
Command: "Cortex: Add tag to current file"
Enter: "issue-123"

Open: tests/auth.test.ts
Command: "Cortex: Add tag to current file"
Enter: "issue-123"

Open: src/api/endpoints.ts
Command: "Cortex: Add tag to current file"
Enter: "issue-456"
```

#### Tag by status
```
Open: src/auth/login.ts
Command: "Cortex: Add tag to current file"
Enter: "in-progress"

Open: src/api/endpoints.ts
Command: "Cortex: Add tag to current file"
Enter: "review-needed"
```

#### Result

**By Tag (Issue Tracking)**:
```
issue-123 (2 files)
├── src/auth/login.ts
└── tests/auth.test.ts

issue-456 (1 file)
└── src/api/endpoints.ts
```

**By Tag (Status)**:
```
in-progress (1 file)
└── src/auth/login.ts

review-needed (1 file)
└── src/api/endpoints.ts
```

### Benefit
- **Issue-based navigation**: Instantly see all files related to an issue
- **Status tracking**: Know what needs review without opening files
- **No code pollution**: Metadata separate from source code

---

## Scenario 3: Academic Research Project

### Problem
PhD student managing papers, datasets, code, and drafts across multiple research threads.

### Files
```
workspace/
├── papers/
│   ├── smith-2020.pdf
│   ├── jones-2021.pdf
│   └── chen-2023.pdf
├── data/
│   ├── experiment-1.csv
│   ├── experiment-2.csv
│   └── analysis.py
└── drafts/
    └── chapter-3.md
```

### Cortex Approach

#### Organize by research thread
```
Papers on neural networks:
- Assign "thread-neural-nets" to smith-2020.pdf
- Assign "thread-neural-nets" to jones-2021.pdf

Papers on optimization:
- Assign "thread-optimization" to chen-2023.pdf

Cross-cutting code:
- Assign both "thread-neural-nets" AND "thread-optimization" to analysis.py
```

#### Tag by reading status
```
Tag smith-2020.pdf with "read"
Tag jones-2021.pdf with "to-read"
Tag chen-2023.pdf with "cited-in-chapter-3"
```

#### Result

**By Context**:
```
thread-neural-nets (3 items)
├── papers/smith-2020.pdf
├── papers/jones-2021.pdf
└── data/analysis.py

thread-optimization (2 items)
├── papers/chen-2023.pdf
└── data/analysis.py
```

**By Tag**:
```
read (1 item)
└── papers/smith-2020.pdf

to-read (1 item)
└── papers/jones-2021.pdf

cited-in-chapter-3 (1 item)
└── papers/chen-2023.pdf
```

**By Type**:
```
pdf (3 items)
├── papers/smith-2020.pdf
├── papers/jones-2021.pdf
└── papers/chen-2023.pdf

python (1 item)
└── data/analysis.py

data (2 items)
├── data/experiment-1.csv
└── data/experiment-2.csv
```

### Benefit
- **Thread-based organization**: See all materials for one research angle
- **Reading workflow**: Track what you've read vs. need to read
- **Cross-references**: Link papers to chapters where they're cited

---

## Scenario 4: Frontend Developer with Design Handoffs

### Problem
Designer sends mockups. You create components. Need to track which components match which designs.

### Cortex Approach

#### Tag components by design status
```
Open: src/components/Button.tsx
Command: "Cortex: Add tag to current file"
Enter: "design-approved"

Open: src/components/Modal.tsx
Command: "Cortex: Add tag to current file"
Enter: "awaiting-design"

Open: src/components/Form.tsx
Command: "Cortex: Add tag to current file"
Enter: "design-in-progress"
```

#### Link designs to components via context
```
Open: designs/homepage-v2.fig
Command: "Cortex: Assign context to current file"
Enter: "homepage-redesign"

Open: src/pages/Home.tsx
Command: "Cortex: Assign context to current file"
Enter: "homepage-redesign"

Open: src/components/Hero.tsx
Command: "Cortex: Assign context to current file"
Enter: "homepage-redesign"
```

#### Result

**By Tag (Design Status)**:
```
design-approved (1 file)
└── src/components/Button.tsx

awaiting-design (1 file)
└── src/components/Modal.tsx

design-in-progress (1 file)
└── src/components/Form.tsx
```

**By Context (Project)**:
```
homepage-redesign (3 files)
├── designs/homepage-v2.fig
├── src/pages/Home.tsx
└── src/components/Hero.tsx
```

### Benefit
- **Design workflow**: See which components need designer attention
- **Project view**: See design files + code in one place
- **Handoff tracking**: Mark when design is approved

---

## Scenario 5: Freelancer Managing Multiple Gigs

### Problem
5 small projects, all in one workspace. Need to invoice separately and track hours per project.

### Cortex Approach

#### One context per client
```
Context: "gig-startup-x"
Context: "gig-agency-y"
Context: "gig-personal-site"
Context: "gig-nonprofit-z"
Context: "gig-consulting-a"
```

#### Tag by billing status
```
Tag files with "billed"
Tag files with "unbilled"
Tag files with "invoiced"
```

#### Result

**By Context**:
```
gig-startup-x (12 files)
├── projects/startup-x/code/...
└── invoices/startup-x-jan.pdf

gig-agency-y (5 files)
├── projects/agency-y/...
└── ...
```

**By Tag (Billing)**:
```
unbilled (8 files)
├── projects/startup-x/feature-new.ts
└── ...

invoiced (15 files)
├── invoices/startup-x-jan.pdf
└── ...
```

### Benefit
- **Client isolation**: See only one client's files
- **Billing workflow**: Track what's been billed
- **End-of-month**: Quickly find all unbilled work

---

## Advanced Patterns

### Pattern 1: Hierarchical Contexts

Use naming conventions:
```
project-alpha
project-alpha-frontend
project-alpha-backend
project-alpha-infra
```

Cortex doesn't enforce hierarchy, but you can filter mentally or via naming.

### Pattern 2: Time-Based Tags

Tag files with dates:
```
"2024-q1"
"2024-q2"
"sprint-15"
```

Useful for:
- Quarterly reviews
- Sprint planning
- Archival

### Pattern 3: Priority Tags

```
"p0" (critical)
"p1" (high)
"p2" (medium)
"p3" (low)
```

Quickly see all P0 items across all projects.

### Pattern 4: Skill-Based Tags

For learning or team coordination:
```
"needs-react-expert"
"needs-sql-review"
"beginner-friendly"
```

### Pattern 5: Lifecycle Tags

```
"draft"
"ready-for-review"
"approved"
"archived"
```

Track document/code maturity.

---

## Tips for Effective Use

1. **Start small**: Tag 5-10 files, see how it feels
2. **Use consistent naming**: `project-x`, not `projectX` or `Project X`
3. **Don't over-tag**: 2-3 tags per file is usually enough
4. **Use contexts for "what"**: What project/client/domain
5. **Use tags for "why"**: Why you care (urgent, review, learning)
6. **Rebuild periodically**: If index feels stale, rebuild

---

## Integration Ideas (Future)

### With Git
- Tag files modified in last commit: `recent-change`
- Tag files with conflicts: `needs-merge`

### With Tasks
- Tag files mentioned in TODOs
- Link to issue trackers (Jira, GitHub Issues)

### With Teams
- Commit `.cortex/` to Git for shared contexts
- Team conventions: `@person-name` for ownership tags

### With Time
- Auto-tag based on modification time
- Archive contexts older than X months

---

**Cortex** - The more you use it, the smarter your workspace becomes.
