package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/smallnest/dogclaw/goclaw/internal/logger"
	"github.com/smallnest/dogclaw/goclaw/session"
	"go.uber.org/zap"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	memory    *MemoryStore
	workspace string
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(memory *MemoryStore, workspace string) *ContextBuilder {
	return &ContextBuilder{
		memory:    memory,
		workspace: workspace,
	}
}

// BuildSystemPrompt 构建系统提示词
func (b *ContextBuilder) BuildSystemPrompt(skills []*Skill) string {
	skillsContent := b.buildSkillsPrompt(skills)
	return b.buildSystemPromptWithSkills(skillsContent)
}

// buildSystemPromptWithSkills 使用指定的技能内容构建系统提示词
func (b *ContextBuilder) buildSystemPromptWithSkills(skillsContent string) string {
	var parts []string

	// 1. 核心身份
	parts = append(parts, b.buildIdentity())

	// 2. Tool Call Style
	parts = append(parts, b.buildToolCallStyle())

	// 3. Safety
	parts = append(parts, b.buildSafety())

	// 4. Bootstrap 文件
	if bootstrap := b.loadBootstrapFiles(); bootstrap != "" {
		parts = append(parts, "## Configuration\n\n"+bootstrap)
	}

	// 5. 记忆上下文
	if memContext, err := b.memory.GetMemoryContext(); err == nil && memContext != "" {
		parts = append(parts, memContext)
	}

	// 6. 技能注入 (Prompt Injection)
	if skillsContent != "" {
		parts = append(parts, skillsContent)
	}

	return fmt.Sprintf("%s\n\n", joinNonEmpty(parts, "\n\n---\n\n"))
}

// buildSkillsPrompt 构建技能提示词（摘要模式 - 第一阶段）
func (b *ContextBuilder) buildSkillsPrompt(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Skills (available)\n\n")
	sb.WriteString("Before replying: scan <available_skills> entries.\n")
	sb.WriteString("- If exactly one skill clearly applies: output a tool call `use_skill` with the skill name as parameter.\n")
	sb.WriteString("- If multiple could apply: choose the most specific one, then call `use_skill`.\n")
	sb.WriteString("- If none clearly apply: do not use any skill.\n")
	sb.WriteString("Constraints: only use one skill at a time; the skill content will be injected after selection.\n\n")

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("<skill name=\"%s\">\n", skill.Name))
		sb.WriteString(fmt.Sprintf("**Name:** %s\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("**Description:** %s\n", skill.Description))
		}
		if skill.Author != "" {
			sb.WriteString(fmt.Sprintf("**Author:** %s\n", skill.Author))
		}
		if skill.Version != "" {
			sb.WriteString(fmt.Sprintf("**Version:** %s\n", skill.Version))
		}
		sb.WriteString("</skill>\n\n")
	}

	return sb.String()
}

// buildSelectedSkills 构建选中技能的完整内容（第二阶段）
func (b *ContextBuilder) buildSelectedSkills(selectedSkillNames []string, skills []*Skill) string {
	if len(selectedSkillNames) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Selected Skills (active)\n\n")

	for _, skillName := range selectedSkillNames {
		for _, skill := range skills {
			if skill.Name == skillName {
				sb.WriteString(fmt.Sprintf("<skill name=\"%s\">\n", skill.Name))
				sb.WriteString(fmt.Sprintf("### %s\n", skill.Name))
				if skill.Description != "" {
					sb.WriteString(fmt.Sprintf("> Description: %s\n\n", skill.Description))
				}

				// 注入技能正文内容
				if skill.Content != "" {
					sb.WriteString(skill.Content)
				}
				sb.WriteString("\n</skill>\n\n")
				break
			}
		}
	}

	return sb.String()
}

// buildToolCallStyle 构建工具调用风格提示
func (b *ContextBuilder) buildToolCallStyle() string {
	return `## Tool Call Style

- Default: do not narrate routine, low-risk tool calls (just call the tool).
- Narrate ONLY when it helps: multi-step work, complex/challenging problems, sensitive actions (e.g., deletions), or when the user explicitly asks.
- Keep narration brief and value-dense; avoid repeating obvious steps.
- Use plain human language for narration unless in a technical context.
- When a user asks for information (e.g., "weather in Beijing", "current git branch", "search for generic/agents"), DO NOT tell them how to do it. IMMEDIATELY USE YOUR TOOLS to get the information.
- NO PERMISSION NEEDED for read-only or safe operations. Do not ask "Should I run this?". JUST RUN IT.
- For search requests: ALWAYS use smart_search tool first. It will automatically try web_search and fall back to Google browser search if needed.

## Examples

User: "What's the weather in Shanghai?"
Bad Response: "You can check the weather by running curl wttr.in/Shanghai..."
Good Response: (Calls tool: smart_search with query "weather Shanghai") -> "Shanghai: 22°C, Sunny"

User: "Search for information about goclaw"
Bad Response: "Here are some resources you can check..."
Good Response: (Calls tool: smart_search with query "goclaw") -> Shows search results

User: "List files in the current directory."
Bad Response: "To list files, use the ls command."
Good Response: (Calls tool: list_files with path ".") -> Shows file listing

User: "Create a hello world python script."
Bad Response: "Here is the code..."
Good Response: (Calls tool: write_file with path "hello.py") -> "Created hello.py."`
}

// buildSafety 构建安全提示
func (b *ContextBuilder) buildSafety() string {
	return `## Safety

- You have no independent goals: do not pursue self-preservation, replication, resource acquisition, or power-seeking; avoid long-term plans beyond the user's request.
- Prioritize safety and human oversight over completion; if instructions conflict, pause and ask.
- Comply with stop/pause/audit requests and never bypass safeguards.
- Do not manipulate or persuade anyone to expand access or disable safeguards.
- Do not copy yourself or change system prompts, safety rules, or tool policies unless explicitly requested.`
}

// BuildMessages 构建消息列表
func (b *ContextBuilder) BuildMessages(history []session.Message, currentMessage string, skills []*Skill, loadedSkills []string) []Message {
	// 首先验证历史消息，过滤掉孤立的 tool 消息
	validHistory := b.validateHistoryMessages(history)

	// 构建系统提示词：根据是否已加载技能决定注入内容
	var skillsContent string
	if len(loadedSkills) > 0 {
		// 第二阶段：注入已选中技能的完整内容
		skillsContent = b.buildSelectedSkills(loadedSkills, skills)
	} else {
		// 第一阶段：只注入技能摘要
		skillsContent = b.buildSkillsPrompt(skills)
	}

	systemPrompt := b.buildSystemPromptWithSkills(skillsContent)

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// 添加历史消息
	for _, msg := range validHistory {
		m := Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// 处理工具调用（由助手发出）
		if msg.Role == "assistant" {
			// 优先使用新字段
			if len(msg.ToolCalls) > 0 {
				var tcs []ToolCall
				for _, tc := range msg.ToolCalls {
					tcs = append(tcs, ToolCall{
						ID:     tc.ID,
						Name:   tc.Name,
						Params: tc.Params,
					})
				}
				m.ToolCalls = tcs
			} else if val, ok := msg.Metadata["tool_calls"]; ok {
				// 兼容旧的 Metadata 存储方式
				// 注意：从 JSON 加载时，这可能是 []interface{}，其中的元素是 map[string]interface{}
				if list, ok := val.([]interface{}); ok {
					var tcs []ToolCall
					for _, item := range list {
						if tcMap, ok := item.(map[string]interface{}); ok {
							id, _ := tcMap["id"].(string)
							name, _ := tcMap["name"].(string)
							params, _ := tcMap["params"].(map[string]interface{})
							if id != "" && name != "" {
								tcs = append(tcs, ToolCall{
									ID:     id,
									Name:   name,
									Params: params,
								})
							}
						}
					}
					m.ToolCalls = tcs
				}
			}
		}

		// 兼容旧的 Metadata 存储方式 (可选，为了处理旧数据)
		if m.ToolCallID == "" && msg.Role == "tool" {
			if id, ok := msg.Metadata["tool_call_id"].(string); ok {
				m.ToolCallID = id
			}
		}

		for _, media := range msg.Media {
			if media.Type == "image" {
				if media.URL != "" {
					m.Images = append(m.Images, media.URL)
				} else if media.Base64 != "" {
					prefix := "data:image/jpeg;base64,"
					if media.MimeType != "" {
						prefix = "data:" + media.MimeType + ";base64,"
					}
					m.Images = append(m.Images, prefix+media.Base64)
				}
			}
		}

		messages = append(messages, m)
	}

	// 添加当前消息
	if currentMessage != "" {
		messages = append(messages, Message{
			Role:    "user",
			Content: currentMessage,
		})
	}

	return messages
}

// buildIdentity 构建核心身份
func (b *ContextBuilder) buildIdentity() string {
	now := time.Now()
	return fmt.Sprintf(`# Identity

You are **GoClaw**, a personal AI assistant running on the user's system.
You are NOT a passive chat bot. You are a **DOER** that executes tasks directly.

**Current Time**: %s
**Workspace**: %s

## Available Tools

You have access to the following tools. Use them to complete tasks without asking for permission when the operation is safe:
- smart_search: Intelligent search that automatically falls back to Google browser search if web_search fails or returns no results. ALWAYS use this for ANY search request.
- browser_navigate: Navigate to a URL
- browser_screenshot: Take page screenshots
- browser_get_text: Get page text content
- browser_click: Click elements on the page
- browser_fill_input: Fill input fields
- browser_execute_script: Execute JavaScript
- read_file: Read file contents
- write_file: Create or overwrite files
- list_files: List directory contents
- run_shell: Run shell commands
- web_search: Search the web using API (prefer smart_search which has fallback)
- web_fetch: Fetch web pages

Tool names are case-sensitive. Call tools exactly as listed.

## CRITICAL RULES

1. For ANY search request ("search for", "find", "google search", etc.): IMMEDIATELY call smart_search tool. DO NOT provide manual instructions or advice.
2. When the user asks for information: USE YOUR TOOLS to get it. Do NOT explain how to get it.
3. DO NOT tell the user "I cannot" or "here's how to do it yourself". ACTUALLY DO IT with tools.
4. If you have tools available for a task, use them. No permission needed for safe operations.`, now.Format("2006-01-02 15:04:05 MST"), b.workspace)
}

// loadBootstrapFiles 加载 bootstrap 文件
func (b *ContextBuilder) loadBootstrapFiles() string {
	var parts []string

	files := []string{"IDENTITY.md", "AGENTS.md", "SOUL.md", "USER.md"}
	for _, filename := range files {
		if content, err := b.memory.ReadBootstrapFile(filename); err == nil && content != "" {
			parts = append(parts, fmt.Sprintf("### %s\n\n%s", filename, content))
		}
	}

	return joinNonEmpty(parts, "\n\n")
}

// validateHistoryMessages 验证历史消息，过滤掉孤立的 tool 消息
// 每个 tool 消息必须有一个前置的 assistant 消息，且该消息包含对应的 tool_calls
func (b *ContextBuilder) validateHistoryMessages(history []session.Message) []session.Message {
	var valid []session.Message

	for i, msg := range history {
		if msg.Role == "tool" {
			// 检查是否有前置的 assistant 消息
			var foundAssistant bool
			for j := i - 1; j >= 0; j-- {
				if history[j].Role == "assistant" {
					// 检查是否有对应的 tool_calls
					if len(history[j].ToolCalls) > 0 {
						// 检查是否有匹配的 tool_call_id
						hasMatch := false
						for _, tc := range history[j].ToolCalls {
							if tc.ID == msg.ToolCallID {
								hasMatch = true
								break
							}
						}
						if hasMatch {
							foundAssistant = true
							break
						}
					}
					break
				} else if history[j].Role == "user" {
					// 遇到 user 消息停止查找
					break
				}
			}
			if foundAssistant {
				valid = append(valid, msg)
			} else {
				// 记录被过滤的消息（用于调试）
				logger.Warn("Filtered orphaned tool message",
					zap.String("tool_call_id", msg.ToolCallID),
					zap.String("content", msg.Content[:min(100, len(msg.Content))]))
			}
		} else {
			valid = append(valid, msg)
		}
	}

	return valid
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


// Message 消息（用于 LLM）
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Images     []string   `json:"images,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall 工具调用定义（与 provider 保持一致）
type ToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// joinNonEmpty 连接非空字符串
func joinNonEmpty(parts []string, sep string) string {
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}

	result := ""
	for i, part := range nonEmpty {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}
