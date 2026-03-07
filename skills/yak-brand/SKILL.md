---
name: yak-brand
description: Stamping the work. Create a git commit based on instructions
allowed-tools: Bash(git:*), Read, Write, Edit, Grep, Glob
argument-hint: [instructions for what to commit]
---

## Create Git Commit

User instructions: $ARGUMENTS

### Writing Style Guide: Your Voice

#### Core Characteristics
- **Conversational yet technical** - Mix informal language with precise technical terms
- **Action-oriented** - Focus on what's being done, what needs doing, or what should happen next
- **Collaborative tone** - Use "we" frequently, ask for input, suggest pairing
- **Pragmatically humorous** - Slip in quirky phrases ("yak shaving", "commandeer") without overdoing it

#### Sentence Structure
- **Variable length** - Mix short declarations with longer explanatory sentences
- **Start with context** - Often begin with "Given that...", "When you have...", or temporal markers
- **Use semicolons liberally** - Connect related thoughts with semicolons rather than separate sentences
- **Numbered lists for clarity** - When presenting multiple points, use numbered lists

#### Vocabulary Choices
- **Tech-casual blend**: "fancy picking this up", "semi-validated", "non-trivial amount of effort"
- **Specific jargon without explanation**: K8s, helm chart, RFC, MVM - assume technical literacy
- **Understated descriptions**: "clearly things we can do" instead of "major improvements needed"
- **Active verbs**: commandeer, resurrect, calibrate, backfill

#### Formatting Patterns
- **Inline questions** - Embed questions naturally in text flow
- **Parenthetical asides** - Add clarifying details in parentheses
- **Time stamps** - Include specific times when relevant (18:58)
- **Tagging people** - Use @ mentions liberally

#### Tone Markers
- Avoid excessive enthusiasm - "Big YES!" is your maximum excitement level
- End with concrete next steps or open questions
- Acknowledge work done while pivoting to what's next
- Self-deprecating about challenges ("5 yaks shaved... clearly more yaks")

### Process:

1. **Interpret user's intent** from: $ARGUMENTS
   Examples:
   - "everything related to the lazygit change just made"
   - "only the authentication files"
   - "the bug fix we just discussed but not the refactoring"
   - "all changes except tests"
   - "the performance improvements to the API"

2. **Check current status**:
   - Run `git status --porcelain` to see all changes
   - Understand what's modified, staged, and untracked

3. **Stage appropriate files**:
   - Use `git add` for specific files matching the intent
   - Use `git reset` to unstage files that don't match
   - Group related changes logically

4. **Generate commit message** following **Clean Commit**:
   - Format: `<emoji> <type>: <description>`
   - Types: new, update, remove, security, setup, chore, test, docs, release
   - Keep description under 72 characters
   - Add body if needed for context (blank line after subject)
   - Always include "Co-Authored-By: Claude <noreply@anthropic.com>" at the end

5. **Create the commit**:
   - Use `git commit -m` with the generated message
   - For multi-line messages, use proper quoting

6. **Report results**:
   ```
   Committed X files:
   - path/to/file1
   - path/to/file2

   Left uncommitted (if any):
   - path/to/file3

   Commit created:
   ----------------------------------------
   🔧 update (auth): implement OAuth2 refresh token flow

   Added JWT refresh token rotation with secure storage
   and automatic renewal before expiration.

   Co-Authored-By: Claude <noreply@anthropic.com>
   ----------------------------------------

   Run 'git log -1' to view the commit
   ```

### Clean Commit Types:

| Emoji | Type | Use for |
|-------|------|---------|
| 📦 | new | New feature or file |
| 🔧 | update | Update existing feature or code |
| 🗑️ | remove | Remove code or file |
| 🔒 | security | Security fix or improvement |
| ⚙️ | setup | Setup or configuration change |
| ☕ | chore | Maintenance (deps, build, misc) |
| 🧪 | test | Adding or updating tests |
| 📖 | docs | Documentation only |
| 🚀 | release | Release-related changes |

### Smart Staging Examples:

If user says: "only the config changes"
- Stage: *.config, *.yaml, *.yml, *.json, *.toml, .env files
- Message: "☕ chore (config): update configuration files"

If user says: "the refactoring we just did"
- Use conversation context to identify refactored files
- Message: "🔧 update (module): extract logic to separate functions"

If user says: "everything"
- Stage all changes with `git add -A`
- Message based on the predominant change type

### Implementation Steps:

1. Run `git status --porcelain` to see all changes
2. Interpret user instructions to determine which files to stage
3. Stage appropriate files with `git add [files]`
4. Generate clean-commit message based on staged changes and user intent
5. Create commit using `git commit -m "message"` (or with heredoc for multi-line)
6. Report what was committed and what remains uncommitted
7. Show the created commit message for confirmation
