# Slack Integration Setup Guide

This guide walks through setting up Slack Socket Mode integration for OpenClaw, allowing you to interact with your orchestrator via Slack direct messages.

## Overview

OpenClaw supports Slack integration using Socket Mode, which enables real-time bidirectional communication without requiring a publicly accessible webhook endpoint. This is perfect for development and internal use.

## Prerequisites

- Access to a Slack workspace where you can create apps
- OpenClaw gateway installed and running
- Access to `~/.openclaw/openclaw.json` configuration file

## Part 1: Create and Configure Slack App

### Step 1: Create New Slack App

1. Navigate to https://api.slack.com/apps
2. Click **"Create New App"**
3. Choose **"From scratch"**
4. Enter app details:
   - **App Name**: `Yakob Bot` (or `OpenClaw` or any name you prefer)
   - **Workspace**: Select your workspace
5. Click **"Create App"**

### Step 2: Enable Socket Mode

Socket Mode allows your app to connect to Slack without needing a public URL.

1. In your app settings, go to **Settings → Socket Mode** (left sidebar)
2. Toggle **"Enable Socket Mode"** to **ON**
3. You'll be prompted to create an App-Level Token:
   - **Token Name**: `Socket Token` (or any name)
   - **Scope**: Select `connections:write`
   - Click **"Generate"**
4. **IMPORTANT**: Copy and save the token immediately!
   - Format: `xapp-1-AXXXXXXXXXX-XXXXXXXXXXXXX-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`
   - This is your **App Token** - you'll need it later
   - You won't be able to see it again after closing the dialog
5. Click **"Done"**

### Step 3: Configure OAuth & Permissions

1. Go to **Features → OAuth & Permissions** (left sidebar)
2. Scroll down to **"Bot Token Scopes"**
3. Click **"Add an OAuth Scope"** and add these scopes:
   - `app_mentions:read` - View messages that directly mention your bot
   - `chat:write` - Send messages as your bot
   - `im:history` - View messages in direct messages with your bot
   - `im:read` - View basic info about direct messages with your bot
   - `im:write` - Start direct messages with people
   - `users:read` - View user information for proper user resolution

### Step 4: Enable Messages Tab

Direct messaging requires enabling the Messages Tab in App Home.

1. Go to **Features → App Home** (left sidebar)
2. Scroll down to the **"Show Tabs"** section
3. Under **"Messages Tab"**, toggle it **ON**
4. Make sure **"Allow users to send Slash commands and messages from the messages tab"** is checked

### Step 5: Enable Event Subscriptions

1. Go to **Features → Event Subscriptions** (left sidebar)
2. Toggle **"Enable Events"** to **ON**
3. Scroll down to **"Subscribe to bot events"**
4. Click **"Add Bot User Event"** and add:
   - `message.im` - Listen for messages in direct messages
   - `app_mention` - Listen for mentions of your bot in channels
5. Click **"Save Changes"** at the bottom

### Step 6: Install App to Workspace

1. Go to **Settings → Install App** (left sidebar)
2. Click **"Install to Workspace"**
3. Review the permissions and click **"Allow"**
4. **IMPORTANT**: Copy and save the **Bot User OAuth Token**!
   - Format: `xoxb-XXXXXXXXXXXXX-XXXXXXXXXXXXX-XXXXXXXXXXXXXXXXXXXXXXXX`
   - This is your **Bot Token** - you'll need it later
   - You can always come back to this page to view it again

### Step 7: Get Your Slack User ID

You'll need your Slack User ID to configure the allowlist for DMs.

1. In Slack, click your profile picture (top right)
2. Select **"Profile"**
3. Click the three dots menu (•••) and select **"Copy member ID"**
4. **Save this ID** - it will look like: `U01234ABCD`

Alternatively, you can get it from the URL when viewing your profile - it's the part after `/team/`.

## Part 2: Configure OpenClaw

### Step 8: Add Tokens to Systemd Service (Recommended)

The recommended approach is to store tokens as environment variables in the systemd service file. This keeps secrets out of configuration files.

1. Edit the systemd service override file:

```bash
sudo nano /etc/systemd/system/openclaw-gateway.service.d/override.conf
```

2. Add these lines to the `[Service]` section:

```ini
[Service]
Environment="SLACK_APP_TOKEN=xapp-1-A01234567890-1234567890123-..."
Environment="SLACK_BOT_TOKEN=xoxb-1234567890123-1234567890123-..."
```

3. Replace the placeholder values with your actual tokens:
   - `SLACK_APP_TOKEN`: Your App-Level Token (starts with `xapp-`)
   - `SLACK_BOT_TOKEN`: Your Bot User OAuth Token (starts with `xoxb-`)

4. Save the file and reload systemd:

```bash
sudo systemctl daemon-reload
```

### Step 9: Add Slack Configuration to openclaw.json

Now configure OpenClaw to use Slack. OpenClaw will automatically read the tokens from the environment variables.

1. Open `~/.openclaw/openclaw.json` in your editor
2. Add a `channels` section at the root level (same level as `agents`, `tools`, etc.):

```json
{
  "meta": { ... },
  "wizard": { ... },
  "auth": { ... },
  "agents": { ... },
  "tools": { ... },
  "messages": { ... },
  "commands": { ... },
  "cron": { ... },
  "gateway": { ... },
  "channels": {
    "slack": {
      "enabled": true,
      "mode": "socket",
      "groupPolicy": "open",
      "dm": {
        "enabled": true,
        "policy": "allowlist",
        "allowFrom": ["U01234ABCD"]
      }
    }
  },
  "plugins": {
    "entries": {
      "slack": {
        "enabled": true
      }
    }
  }
}
```

3. Replace `U01234ABCD` with your Slack User ID
4. Save the file

**Channel mentions**: The `groupPolicy: "open"` setting allows the bot to respond to mentions in any channel it's invited to. You can also use:
- `"allowlist"` - Only respond in specific channels (configure `channels.slack.channels`)
- `"disabled"` - Disable channel mentions entirely

**Note**: The tokens are not in the JSON file - OpenClaw reads them from the `SLACK_APP_TOKEN` and `SLACK_BOT_TOKEN` environment variables set in the systemd service.

### Configuration Options Explained

**Core Settings:**
- `channels.slack.enabled`: Set to `true` to activate Slack integration
- `channels.slack.mode`: Must be `"socket"` for Socket Mode (vs webhook mode)
- `plugins.entries.slack.enabled`: Enable the Slack plugin

**Direct Message Settings:**
- `channels.slack.dm.enabled`: Allow direct messages to the bot
- `channels.slack.dm.policy`: `"allowlist"` restricts DMs to specific users
- `channels.slack.dm.allowFrom`: Array of Slack User IDs allowed to DM the bot

**Channel Mention Settings:**
- `channels.slack.groupPolicy`: Controls channel access
  - `"open"`: Bot responds to mentions in any channel it's invited to
  - `"allowlist"`: Bot only responds in specific channels
  - `"disabled"`: Bot doesn't respond in channels at all

**Tokens** are read from environment variables:
- `SLACK_APP_TOKEN`: App-Level Token with `connections:write` scope
- `SLACK_BOT_TOKEN`: Bot User OAuth Token with messaging permissions

### Step 10: Restart OpenClaw Gateway

After updating the configuration, restart the gateway service:

```bash
sudo systemctl restart openclaw-gateway
```

Check the service status to ensure it started successfully:

```bash
systemctl status openclaw-gateway
```

Look for log messages indicating the Slack connection was established.

## Part 3: Test the Integration

### Step 11: Verify Slack Connection

Check the logs to verify Slack connected successfully:

```bash
journalctl -u openclaw-gateway -n 20 | grep slack
```

You should see messages like:
```
[slack] [default] starting provider
[slack] users resolved: U08HZ8ABDV1→U08HZ8ABDV1
[slack] socket mode connected
```

### Step 12: Send a Test Message

1. Open Slack and find your bot in the Apps section (left sidebar)
2. Click on your bot to open a direct message
3. Send a simple message like: `Hello!` or `What's the status?`
4. Your bot should respond!

If you don't see a response:
- Check the gateway logs: `journalctl -u openclaw-gateway -f`
- Verify your tokens are set in the systemd service file
- Make sure your User ID is in the `allowFrom` array
- Confirm Socket Mode is enabled in your Slack app settings
- Verify the Slack plugin is enabled in `openclaw.json`

## Troubleshooting

### Bot doesn't respond to DMs

**Check configuration:**
```bash
# Verify Slack is enabled in openclaw.json
grep -A 10 '"slack"' ~/.openclaw/openclaw.json

# Check environment variables are set
systemctl cat openclaw-gateway | grep SLACK

# Check gateway logs for Slack connection
journalctl -u openclaw-gateway -n 50 | grep slack
```

**Common issues:**
- Tokens not set in systemd service file
- Wrong tokens copied (extra spaces, truncated)
- User ID not in allowlist
- Slack plugin not enabled in `openclaw.json`
- Socket Mode not enabled in Slack app
- Gateway service not restarted after config change
- Need to run `systemctl daemon-reload` after editing service file

### "Invalid token" errors in logs

- Verify you copied the complete token (they're very long!)
- Check tokens in systemd service file: `systemctl cat openclaw-gateway`
- Make sure you're using the **App-Level Token** for `SLACK_APP_TOKEN` (starts with `xapp-`)
- Make sure you're using the **Bot Token** for `SLACK_BOT_TOKEN` (starts with `xoxb-`)
- Tokens may have been revoked - regenerate in Slack app settings
- After updating tokens, run `sudo systemctl daemon-reload` and restart the service

### "missing_scope" errors in logs

- This means you're missing the `users:read` scope
- Go to your Slack app settings → OAuth & Permissions
- Add the `users:read` scope under Bot Token Scopes
- Reinstall the app to workspace
- Restart the gateway service

### Gateway fails to start

```bash
# Check for JSON syntax errors
python3 -m json.tool ~/.openclaw/openclaw.json > /dev/null

# View detailed error logs
journalctl -u openclaw-gateway -xe
```

### Bot connected but not receiving messages

- Verify Event Subscriptions are enabled
- Confirm `message.im` event is subscribed
- Check that bot scopes include `im:history` and `im:read`
- Try reinstalling the app to workspace

## Security Considerations

### Token Security

- **Tokens are stored in systemd service file** - not in version-controlled config files
- The systemd override file at `/etc/systemd/system/openclaw-gateway.service.d/override.conf` should have restricted permissions (root only)
- Tokens provide full access to your Slack workspace - treat them like passwords
- Rotate tokens periodically
- Never commit tokens to git repositories

### Access Control

- The `allowFrom` allowlist prevents unauthorized users from accessing your bot
- Add User IDs carefully - each person can execute commands on your system
- Consider using a dedicated Slack workspace for development
- Monitor gateway logs for suspicious activity

### Network Security

- Socket Mode doesn't require opening inbound ports
- All connections are outbound from your server to Slack
- Uses WSS (WebSocket Secure) encryption
- Still requires proper system-level security (firewall, SSH keys, etc.)

## Advanced Configuration

### Multiple Allowed Users

Add multiple User IDs to allow a team to interact with the bot:

```json
"allowFrom": ["U01234ABCD", "U56789EFGH", "U11111XYZT"]
```

### Channel Mentions

Channel mentions are enabled with `groupPolicy: "open"` in the configuration (see Step 9).

To use the bot in channels:

1. Invite the bot to a channel: `/invite @YakobBot`
2. Mention it in the channel: `@YakobBot what's the status?`
3. The bot will respond to the mention

**Important: Mentions Required for All Messages**

By default, the bot requires an explicit mention (`@YakobBot`) to respond in channels - **including thread replies**. This prevents it from responding to every message in busy channels.

**Thread behavior:**
- **First message**: `@YakobBot Hi! Can you help?` → Bot responds
- **Thread reply**: `@YakobBot What's the status?` → Bot responds (mention required)
- **Thread reply without mention**: `What about now?` → Bot does NOT respond

You must mention the bot in each message, even within a thread, for it to respond in channels. This is by design to keep the bot from being too noisy in group conversations.

**Restricting to specific channels**: If you want to limit which channels the bot can respond in, use `groupPolicy: "allowlist"` and configure specific channels:

```json
"channels": {
  "slack": {
    "groupPolicy": "allowlist",
    "channels": {
      "C0AF4QF10DA": {}
    }
  }
}
```

### Hardcoding Tokens (Not Recommended)

While the recommended approach is to use environment variables (as shown in Step 7), you can also hardcode tokens directly in `openclaw.json`:

```json
"channels": {
  "slack": {
    "enabled": true,
    "mode": "socket",
    "appToken": "xapp-1-A01234567890-...",
    "botToken": "xoxb-1234567890123-...",
    "dm": {
      "enabled": true,
      "policy": "allowlist",
      "allowFrom": ["U01234ABCD"]
    }
  }
}
```

**Warning**: This approach is less secure because:
- Tokens may accidentally be committed to version control
- Config files are often shared or backed up
- Harder to rotate tokens across multiple environments

Use environment variables instead for production deployments.

## Advanced: Daily Summary Cron Job

You can configure OpenClaw to automatically post daily summaries to a Slack channel using cron jobs.

### Configure Daily Summary

The daily summary cron job should already be configured. To update it to post to a specific channel:

```bash
# Get the channel ID (from the channel URL or by right-clicking the channel)
# Example: https://your-workspace.slack.com/archives/C0AF4QF10DA
# Channel ID: C0AF4QF10DA

# Update the cron job to use the message tool
openclaw cron edit <JOB_ID> --message "Summarize today's work and use the message tool to send the summary to slack channel CHANNEL_ID. Include: completed tasks, blocked workers, tomorrow's priorities. Run yx ls and check-workers.sh first."
```

**Important**: The cron job must instruct the agent to use the `message` tool to send to the channel. The built-in `--announce` delivery mechanism may not work reliably for channels.

### Check Existing Cron Jobs

```bash
# List all cron jobs
openclaw cron list

# View run history for a specific job
openclaw cron runs --id <JOB_ID> --limit 5

# Test run a cron job immediately
openclaw cron run <JOB_ID>
```

### Example Daily Summary Configuration

The daily summary typically runs at 17:00 UTC and includes:
- Completed tasks for the day
- Blocked workers status
- Active worker status
- Tomorrow's priorities

The summary is generated by running `yx ls` and `check-workers.sh` to gather current task status.

## Next Steps

Once Slack integration is working:

- Configure notification preferences
- Add slash commands for common operations
- Create custom workflows triggered by Slack messages
- Set up additional cron jobs for periodic updates

## References

- [Slack Socket Mode Documentation](https://api.slack.com/apis/connections/socket)
- [Slack App Management](https://api.slack.com/apps)
- OpenClaw Gateway Configuration Guide
- OpenClaw Channels Documentation
