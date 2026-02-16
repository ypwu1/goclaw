package channels

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/internal/logger"
	"go.uber.org/zap"
)

// coreDataEpochOffset is the offset between Unix epoch (1970-01-01) and
// Core Data epoch (2001-01-01) in nanoseconds.
// macOS Messages stores dates as nanoseconds since 2001-01-01.
const coreDataEpochOffset = 978307200

// IMessageChannel iMessage 通道 (macOS only)
type IMessageChannel struct {
	*BaseChannelImpl
	dbPath       string
	pollInterval time.Duration
	lastRowID    int64
}

// IMessageConfig iMessage 配置
type IMessageConfig struct {
	BaseChannelConfig
	DBPath       string `mapstructure:"db_path" json:"db_path"`
	PollInterval int    `mapstructure:"poll_interval" json:"poll_interval"` // seconds
}

// NewIMessageChannel 创建 iMessage 通道
func NewIMessageChannel(cfg IMessageConfig, msgBus *bus.MessageBus) (*IMessageChannel, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, "Library", "Messages", "chat.db")
	}

	pollInterval := 3 * time.Second
	if cfg.PollInterval > 0 {
		pollInterval = time.Duration(cfg.PollInterval) * time.Second
	}

	return &IMessageChannel{
		BaseChannelImpl: NewBaseChannelImpl("imessage", cfg.AccountID, cfg.BaseChannelConfig, msgBus),
		dbPath:          dbPath,
		pollInterval:    pollInterval,
	}, nil
}

// Start 启动 iMessage 通道
func (c *IMessageChannel) Start(ctx context.Context) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("imessage channel is only supported on macOS (current: %s)", runtime.GOOS)
	}

	if err := c.BaseChannelImpl.Start(ctx); err != nil {
		return err
	}

	// Verify the chat.db file exists
	if _, err := os.Stat(c.dbPath); os.IsNotExist(err) {
		return fmt.Errorf("iMessage database not found at %s", c.dbPath)
	}

	// Initialize lastRowID by querying the current max ROWID
	if err := c.initLastRowID(); err != nil {
		return fmt.Errorf("failed to initialize iMessage database: %w", err)
	}

	logger.Info("Starting iMessage channel",
		zap.String("db_path", c.dbPath),
		zap.Duration("poll_interval", c.pollInterval),
		zap.Int64("last_row_id", c.lastRowID),
	)

	go c.pollMessages(ctx)

	return nil
}

// initLastRowID reads the current max ROWID so we only process new messages
func (c *IMessageChannel) initLastRowID() error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	var maxRowID sql.NullInt64
	err = db.QueryRow("SELECT MAX(ROWID) FROM message").Scan(&maxRowID)
	if err != nil {
		return fmt.Errorf("failed to query max ROWID: %w", err)
	}

	if maxRowID.Valid {
		c.lastRowID = maxRowID.Int64
	}

	return nil
}

// openDB opens the chat.db in read-only mode
func (c *IMessageChannel) openDB() (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL&_busy_timeout=5000", c.dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open iMessage database: %w", err)
	}
	return db, nil
}

// pollMessages 轮询 chat.db 获取新消息
func (c *IMessageChannel) pollMessages(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("iMessage channel stopped by context")
			return
		case <-c.WaitForStop():
			logger.Info("iMessage channel stopped")
			return
		case <-ticker.C:
			if err := c.fetchNewMessages(ctx); err != nil {
				logger.Error("Failed to fetch iMessage messages", zap.Error(err))
			}
		}
	}
}

// iMessageRow represents a row from the chat.db query
type iMessageRow struct {
	RowID          int64
	Text           sql.NullString
	Date           int64
	IsFromMe       int
	Service        sql.NullString
	SenderID       sql.NullString
	ChatIdentifier sql.NullString
}

// fetchNewMessages queries chat.db for messages with ROWID > lastRowID
func (c *IMessageChannel) fetchNewMessages(ctx context.Context) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	query := `
		SELECT
			m.ROWID,
			m.text,
			m.date,
			m.is_from_me,
			m.service,
			h.id AS sender_id,
			c.chat_identifier
		FROM message m
		JOIN chat_message_join cmj ON m.ROWID = cmj.message_id
		JOIN chat c ON cmj.chat_id = c.ROWID
		LEFT JOIN handle h ON m.handle_id = h.ROWID
		WHERE m.ROWID > ? AND m.is_from_me = 0
		ORDER BY m.ROWID ASC
	`

	rows, err := db.QueryContext(ctx, query, c.lastRowID)
	if err != nil {
		return fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row iMessageRow
		if err := rows.Scan(
			&row.RowID,
			&row.Text,
			&row.Date,
			&row.IsFromMe,
			&row.Service,
			&row.SenderID,
			&row.ChatIdentifier,
		); err != nil {
			logger.Error("Failed to scan iMessage row", zap.Error(err))
			continue
		}

		if err := c.handleMessage(ctx, &row); err != nil {
			logger.Error("Failed to handle iMessage",
				zap.Error(err),
				zap.Int64("row_id", row.RowID),
			)
		}

		// Update lastRowID
		if row.RowID > c.lastRowID {
			c.lastRowID = row.RowID
		}
	}

	return rows.Err()
}

// handleMessage processes a single iMessage row
func (c *IMessageChannel) handleMessage(ctx context.Context, row *iMessageRow) error {
	// Skip messages without text
	text := row.Text.String
	if !row.Text.Valid || text == "" {
		return nil
	}

	senderID := row.SenderID.String
	if !row.SenderID.Valid {
		senderID = "unknown"
	}

	// Check permission
	if !c.IsAllowed(senderID) {
		return nil
	}

	chatID := row.ChatIdentifier.String
	if !row.ChatIdentifier.Valid {
		chatID = senderID
	}

	// Convert Core Data timestamp to time.Time
	timestamp := coreDataTimestampToTime(row.Date)

	inboundMsg := &bus.InboundMessage{
		ID:       fmt.Sprintf("imsg_%d", row.RowID),
		Channel:  c.Name(),
		SenderID: senderID,
		ChatID:   chatID,
		Content:  text,
		Metadata: map[string]interface{}{
			"service":  row.Service.String,
			"row_id":   row.RowID,
			"platform": "imessage",
		},
		Timestamp: timestamp,
	}

	return c.PublishInbound(ctx, inboundMsg)
}

// Send 通过 AppleScript 发送 iMessage
func (c *IMessageChannel) Send(msg *bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("imessage channel is not running")
	}

	if runtime.GOOS != "darwin" {
		return fmt.Errorf("imessage send is only supported on macOS")
	}

	recipient := msg.ChatID
	if recipient == "" {
		return fmt.Errorf("recipient (chat_id) is required for iMessage")
	}

	content := msg.Content
	if content == "" {
		return nil
	}

	// Escape special characters for AppleScript
	content = escapeAppleScript(content)
	recipient = escapeAppleScript(recipient)

	script := fmt.Sprintf(`
		tell application "Messages"
			set targetService to 1st account whose service type = iMessage
			set targetBuddy to participant "%s" of account targetService
			send "%s" to targetBuddy
		end tell
	`, recipient, content)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to send iMessage via AppleScript: %w, output: %s", err, string(output))
	}

	logger.Info("iMessage sent",
		zap.String("recipient", msg.ChatID),
		zap.Int("content_length", len(msg.Content)),
	)

	return nil
}

// escapeAppleScript escapes special characters for AppleScript strings
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// coreDataTimestampToTime converts a macOS Core Data timestamp (nanoseconds since 2001-01-01) to time.Time
func coreDataTimestampToTime(timestamp int64) time.Time {
	// macOS Messages stores timestamps as nanoseconds since 2001-01-01
	// Convert to Unix timestamp: add the epoch offset and convert from nanoseconds
	unixNano := timestamp + coreDataEpochOffset*1e9
	return time.Unix(0, unixNano)
}
