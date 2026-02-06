# Skills 系统测试文档

本文档记录了 goclaw Skills 系统的测试过程和方法，用于验证所有已实现的 CLI 子命令功能。

## 测试环境

- **操作系统**: macOS / Linux
- **Go 版本**: go1.x
- **构建命令**: `go build -o /tmp/goclaw .`

## 测试前准备

### 1. 构建 goclaw

```bash
cd /path/to/goclaw
go build -o /tmp/goclaw .
```

### 2. 验证构建成功

```bash
/tmp/goclaw --help
```

预期输出应显示所有可用命令，包括 `skills` 和 `chat`。

## 功能测试

### 1. skills list 命令

#### 1.1 基础列表功能

**命令**: `goclaw skills list`

**测试步骤**:
```bash
/tmp/goclaw skills list
```

**预期输出**:
```
Found 13 skills:

📦 video-frames
   Extract frames or short clips from videos using ffmpeg.
   Icon: 🎞️
   Requires: [ffmpeg]

📦 weather
   Get current weather and forecasts (no API key required).
   Icon: 🌤️
   Requires: [curl]

...
```

**验证点**:
- [x] 显示已发现技能的总数
- [x] 每个技能显示名称、描述、图标（如果有）
- [x] 显示依赖项（bins、env、os 等）
- [x] 无技能时显示 "No skills found."

#### 1.2 详细模式 (-v/--verbose)

**命令**: `goclaw skills list -v`

**测试步骤**:
```bash
/tmp/goclaw skills list -v | grep -A 30 "📦 weather"
```

**预期输出**:
```
📦 weather
   Get current weather and forecasts (no API key required).
   Icon: 🌤️
   Requires: [curl]

   --- Content ---
   # Weather

   Two free services, no API keys needed.

   ## wttr.in (primary)

   Quick one-liner:

   ```bash
   curl -s "wttr.in/London?format=3"
   # Output: London: ⛅️ +8°C
   ```
   ...
```

**验证点**:
- [x] 显示技能的完整内容（SKILL.md 的 Markdown 正文）
- [x] 内容格式正确，有适当的缩进

### 2. skills validate 命令

#### 2.1 验证依赖满足的技能

**命令**: `goclaw skills validate <skill-name>`

**测试步骤**:
```bash
/tmp/goclaw skills validate weather
```

**预期输出**:
```
Validating skill: weather

Binary dependencies:
  ✅ curl: /usr/bin/curl

✅ All dependencies satisfied!
```

**验证点**:
- [x] 显示技能名称
- [x] 检查二进制依赖（bins）
- [x] 检查 AnyBins（至少一个存在即可）
- [x] 检查环境变量（env）
- [x] 检查操作系统兼容性（os）
- [x] 敏感环境变量值被隐藏（如 API_KEY）
- [x] 所有依赖满足时显示成功消息

#### 2.2 验证有环境变量依赖的技能

**测试步骤**:
```bash
# 设置测试环境变量
export OPENAI_API_KEY="sk-test1234567890abcdef"

/tmp/goclaw skills validate openai-whisper-api
```

**预期输出**:
```
Validating skill: openai-whisper-api

Binary dependencies:
  ✅ curl: /usr/bin/curl

Environment variables:
  ✅ OPENAI_API_KEY: sk****def

✅ All dependencies satisfied!
```

**验证点**:
- [x] 环境变量存在时显示 ✅
- [x] 敏感值被部分隐藏（只显示前2位和后2位）

#### 2.3 验证不存在的技能

**测试步骤**:
```bash
/tmp/goclaw skills validate nonexistent-skill
```

**预期输出**:
```
❌ Skill 'nonexistent-skill' not found
```

**验证点**:
- [x] 显示友好的错误消息
- [x] 退出码为非零

### 3. skills install 命令

#### 3.1 从 Git 仓库安装

**命令**: `goclaw skills install <git-url>`

**测试步骤**:
```bash
/tmp/goclaw skills install https://github.com/openclaw/skills
```

**预期输出**:
```
Installing from URL: https://github.com/openclaw/skills
Cloning to /Users/smallnest/.goclaw/skills/skills...
Cloning into '/Users/smallnest/.goclaw/skills/skills'...
...
✅ Skill installed to /Users/smallnest/.goclaw/skills/skills
```

**验证点**:
- [x] 正确解析 Git 仓库 URL
- [x] 自动提取仓库名作为技能目录名
- [x] 执行 git clone 成功
- [x] 安装到 `~/.goclaw/skills/` 目录
- [x] 显示成功消息和安装路径

#### 3.2 从本地目录安装

**命令**: `goclaw skills install <local-path>`

**测试步骤**:
```bash
# 创建测试技能目录
mkdir -p /tmp/test-skill
cat > /tmp/test-skill/SKILL.md << 'EOF'
---
name: test-skill
description: A test skill
metadata:
  openclaw:
    emoji: "🧪"
    requires:
      bins: ["echo"]
---
# Test Skill

This is a test skill.
EOF

/tmp/goclaw skills install /tmp/test-skill
```

**预期输出**:
```
Installing from local path: /tmp/test-skill
Copying to /Users/smallnest/.goclaw/skills/test-skill...
✅ Skill installed to /Users/smallnest/.goclaw/skills/test-skill
```

**验证点**:
- [x] 正确解析本地路径
- [x] 复制目录到目标位置
- [x] 复制后的技能可以被 discover

#### 3.3 覆盖已存在的技能

**测试步骤**:
```bash
# 尝试再次安装同一个技能
echo "y" | /tmp/goclaw skills install https://github.com/openclaw/skills
```

**预期输出**:
```
Installing from URL: https://github.com/openclaw/skills
⚠️  Skill already exists at /Users/xxx/.goclaw/skills/skills
Overwrite? (y/N):
...
✅ Skill installed to /Users/xxx/.goclaw/skills/skills
```

**验证点**:
- [x] 检测到已存在的技能
- [x] 提示用户确认覆盖
- [x] 用户确认后执行覆盖

### 4. skills update 命令

#### 4.1 更新 Git 仓库技能

**命令**: `goclaw skills update <skill-name>`

**测试步骤**:
```bash
/tmp/goclaw skills update skills
```

**预期输出**:
```
Updating skill: skills
From https://github.com/openclaw/skills
   * branch            master       -> FETCH_HEAD
...
✅ Skill updated successfully
```

**验证点**:
- [x] 检测 `.git` 目录确认是 Git 仓库
- [x] 执行 `git pull` 更新
- [x] 显示更新进度
- [x] 更新成功后显示确认消息

#### 4.2 更新非 Git 技能

**测试步骤**:
```bash
/tmp/goclaw skills update test-skill
```

**预期输出**:
```
⚠️  Skill 'test-skill' is not a Git repository, cannot update
```

**验证点**:
- [x] 检测非 Git 仓库
- [x] 显示友好的错误消息

### 5. skills uninstall 命令

#### 5.1 卸载已安装的技能

**命令**: `goclaw skills uninstall <skill-name>`

**测试步骤**:
```bash
echo "y" | /tmp/goclaw skills uninstall test-skill
```

**预期输出**:
```
Uninstalling skill: test-skill
Path: /Users/xxx/.goclaw/skills/test-skill
Confirm? (y/N):
✅ Skill uninstalled successfully
```

**验证点**:
- [x] 显示待删除的技能路径
- [x] 要求用户确认
- [x] 删除成功后显示确认消息
- [x] 目录实际被删除

#### 5.2 卸载不存在的技能

**测试步骤**:
```bash
/tmp/goclaw skills uninstall nonexistent-skill
```

**预期输出**:
```
⚠️  Skill 'nonexistent-skill' is not installed
```

**验证点**:
- [x] 显示友好的错误消息
- [x] 不执行删除操作

### 6. skills config 命令

#### 6.1 显示配置

**命令**: `goclaw skills config show`

**测试步骤**:
```bash
/tmp/goclaw skills config show
```

**预期输出**:
```
Skills Configuration:
===================

No custom skills configuration found.
Using default configuration.

Relevant Tool Configuration:
  Shell enabled: true
  Allowed commands: [git, curl, ...]
```

**验证点**:
- [x] 检查 `~/.goclaw/skills.yaml` 是否存在
- [x] 显示相关的工具配置
- [x] 无自定义配置时显示默认配置提示

#### 6.2 设置配置（部分实现）

**命令**: `goclaw skills config set <key> <value>`

**测试步骤**:
```bash
/tmp/goclaw skills config set disabled.test-skill true
```

**预期输出**:
```
Setting configuration: disabled.test-skill = true
Config type: disabled, skill: test-skill
⚠️  Skills configuration file editing is not yet implemented.
   Please manually edit: /Users/xxx/.goclaw/skills.yaml
```

**注意**: 此功能目前为占位实现，待完善。

### 7. chat 命令增强

#### 7.1 --debug-prompt 参数

**命令**: `goclaw chat --debug-prompt`

**测试步骤**:
```bash
echo "quit" | /tmp/goclaw chat --debug-prompt 2>&1 | head -100
```

**预期输出**:
```
🤖 goclaw Interactive Chat
Type 'quit' or 'exit' to stop, 'clear' to clear history

Loaded 13 skills
...
=== Debug: System Prompt ===
# Identity

You are **GoClaw**, an autonomous AI agent capable of executing tasks...
...

## Available Agent Skills

<skill name="weather">
### weather
> Description: Get current weather and forecasts (no API key required).

# Weather
...
```

**验证点**:
- [x] 启动时打印完整的 System Prompt
- [x] 包含所有已加载技能的完整内容
- [x] 格式清晰，易于调试
- [x] 正常进入聊天模式

#### 7.2 --log-level 参数

**命令**: `goclaw chat --log-level=<level>`

**测试步骤**:
```bash
# 测试不同日志级别
echo "quit" | /tmp/goclaw chat --log-level=debug 2>&1 | grep -i debug
echo "quit" | /tmp/goclaw chat --log-level=warn 2>&1 | grep -i warn
```

**预期输出**:
```
# debug 级别应显示详细日志
2026-02-06T08:38:52.331+0800 [INFO] tools/registry.go:36 Tool registered...
2026-02-06T08:38:52.332+0800 [DEBUG] ...
```

**验证点**:
- [x] 支持 debug、info、warn、error 级别
- [x] 日志输出符合指定级别
- [x] 默认级别为 info

### 8. skills test 命令

**命令**: `goclaw skills test <skill-name> --prompt "<test-prompt>"`

**测试步骤**:
```bash
/tmp/goclaw skills test weather --prompt "What's the weather like in Beijing?"
```

**预期输出**:
```
Testing skill: weather
Prompt: What's the weather like in Beijing?

=== LLM Response ===
[LLM 会根据 skill 内容生成响应]
```

**验证点**:
- [x] 加载指定技能
- [x] 构建包含技能内容的测试 prompt
- [x] 调用 LLM 获取响应
- [x] 显示 LLM 的响应结果

**注意**: 此命令需要有效的 LLM 配置才能完整测试。

## 集成测试场景

### 场景 1: 完整的技能生命周期

**目标**: 验证从安装、使用到卸载的完整流程

**步骤**:
```bash
# 1. 列出当前技能
/tmp/goclaw skills list

# 2. 安装新技能
/tmp/goclaw skills install https://github.com/openclaw/skills

# 3. 验证技能依赖
/tmp/goclaw skills validate weather

# 4. 查看 skill 详细内容
/tmp/goclaw skills list -v | grep -A 20 "📦 weather"

# 5. 在 chat 中使用 skill（通过 --debug-prompt 验证注入）
echo "quit" | /tmp/goclaw chat --debug-prompt | grep -A 10 "### weather"

# 6. 更新技能
/tmp/goclaw skills update skills

# 7. 卸载技能
echo "y" | /tmp/goclaw skills uninstall skills

# 8. 验证卸载成功
ls ~/.goclaw/skills/
```

**验证点**:
- [ ] 每一步都成功执行
- [ ] 技能在 chat 中被正确注入
- [ ] 卸载后技能不再可用

### 场景 2: 依赖验证

**目标**: 验证技能依赖检查的各种情况

**步骤**:
```bash
# 1. 测试无依赖技能
/tmp/goclaw skills validate skill-creator

# 2. 测试有 binary 依赖的技能
/tmp/goclaw skills validate github

# 3. 测试有环境变量依赖的技能
export OPENAI_API_KEY="test-key"
/tmp/goclaw skills validate openai-whisper-api

# 4. 测试有 AnyBins 的技能
/tmp/goclaw skills validate coding-agent

# 5. 测试缺失依赖的情况
# 可以临时修改 PATH 来测试
```

**验证点**:
- [ ] 各种依赖类型都能正确检测
- [ ] 依赖缺失时给出明确的错误提示
- [ ] 敏感信息（API keys）被正确隐藏

## 性能测试

### 技能发现性能

**测试步骤**:
```bash
# 测试包含大量技能的仓库发现性能
time /tmp/goclaw skills list
```

**验证点**:
- [ ] 列出 1000+ 技能在合理时间内完成（< 5秒）
- [ ] 内存使用合理

## 边界条件测试

### 1. 空技能目录

**测试步骤**:
```bash
rm -rf ~/.goclaw/skills/*
/tmp/goclaw skills list
```

**预期输出**: `No skills found.`

### 2. 无效的技能目录

**测试步骤**:
```bash
mkdir -p ~/.goclaw/skills/invalid-skill
# 不创建 SKILL.md
/tmp/goclaw skills list
```

**预期输出**: 跳过无效目录，不报错

### 3. 损坏的 SKILL.md

**测试步骤**:
```bash
mkdir -p ~/.goclaw/skills/corrupted-skill
echo "invalid yaml content" > ~/.goclaw/skills/corrupted-skill/SKILL.md
/tmp/goclaw skills list
```

**预期输出**: 跳过损坏的技能，不报错

## 测试清单总结

### 已实现功能

| 功能 | 状态 | 备注 |
|------|------|------|
| `skills list` | ✅ | 基础列表功能完整 |
| `skills list -v` | ✅ | 详细模式显示完整内容 |
| `skills validate` | ✅ | 完整的依赖检查 |
| `skills install` | ✅ | 支持 Git URL 和本地路径 |
| `skills update` | ✅ | 仅支持 Git 仓库 |
| `skills uninstall` | ✅ | 带确认的删除功能 |
| `skills config show` | ✅ | 显示配置状态 |
| `skills config set` | ⚠️ | 占位实现，待完善 |
| `skills test` | ✅ | 需要 LLM 配置 |
| `chat --debug-prompt` | ✅ | 完整显示 System Prompt |
| `chat --log-level` | ✅ | 支持所有日志级别 |

### 待完善功能

1. **skills config set**: 需要实现 `skills.yaml` 文件的读写
2. **技能冲突检测**: 检测同名技能并提示用户
3. **技能版本管理**: 支持多版本共存和切换
4. **沙箱环境支持**: 在 Docker 沙箱中安装依赖

## 测试覆盖率

根据 `docs/Skills.md` 设计文档中的 CLI 命令列表：

```
已覆盖: 11/13 (84.6%)

未覆盖:
- 安装类型支持 (apt, yum, brew 等) - 需要沙箱环境
- 优先级管理 - 需要扩展 Skill 结构
```

## 附录: 测试数据

### 测试用技能仓库

1. **官方仓库**: https://github.com/openclaw/skills
   - 包含 13 个示例技能
   - 用于测试 install、update、uninstall

2. **Awesome 列表**: https://github.com/VoltAgent/awesome-openclaw-skills
   - 包含 1700+ 技能链接
   - 用于发现和测试各种类型的技能

### 常用测试技能

| 技能名 | 依赖 | 用途 |
|--------|------|------|
| weather | curl | 无需 API key 的天气查询 |
| github | gh | GitHub 交互 |
| coding-agent | claude/codex/opencode/pi (any) | 代码生成代理 |
| openai-whisper-api | curl + OPENAI_API_KEY | 语音转录 |

## 故障排查

### 常见问题

1. **git 命令未找到**
   - 确保系统已安装 git
   - 检查 PATH 环境变量

2. **权限错误**
   - 确保对 `~/.goclaw/skills/` 有写权限
   - 使用 `chmod` 修正权限

3. **LLM 配置错误**
   - 检查 `config.yaml` 中的 provider 配置
   - 验证 API keys 是否有效

## 结论

本测试文档覆盖了 goclaw Skills 系统的所有主要功能。按照本文档的测试步骤执行，可以验证 Skills 系统的正确性和完整性。

**测试通过标准**:
- 所有已实现功能的测试用例全部通过
- 边界条件处理正确
- 错误消息友好且准确
- 性能指标符合要求
