# iMessage 通道

> macOS 平台专属的 iMessage 消息通道

---

## 概述

iMessage 通道允许 GoClaw 通过 macOS 的 iMessage 收发消息。接收消息通过 SQLite 轮询 `~/Library/Messages/chat.db` 实现，发送消息通过 AppleScript（osascript）调用 Messages.app 实现。

### 平台要求

- **仅支持 macOS**（代码可在 Linux 上编译，但运行时需要 macOS 环境）
- 需要 Messages.app 处于运行状态
- 需要「全盘访问」或「自动化」权限以读取 chat.db 和执行 AppleScript

---

## 配置

### 基本配置

在 `~/.goclaw/config.json` 中添加：

```json
{
  "channels": {
    "imessage": {
      "enabled": true,
      "allowed_ids": ["+8613800138000", "user@icloud.com"]
    }
  }
}
```

### 完整配置

```json
{
  "channels": {
    "imessage": {
      "enabled": true,
      "db_path": "",
      "poll_interval": 3,
      "allowed_ids": ["+8613800138000", "user@icloud.com"]
    }
  }
}
```

### 配置项说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `false` | 是否启用 iMessage 通道 |
| `db_path` | string | `~/Library/Messages/chat.db` | iMessage 数据库路径，留空使用默认值 |
| `poll_interval` | int | `3` | 轮询间隔（秒） |
| `allowed_ids` | []string | `[]` | 允许的手机号/邮箱列表，为空则允许所有 |

### 多账号配置

```json
{
  "channels": {
    "imessage": {
      "enabled": true,
      "accounts": {
        "personal": {
          "enabled": true,
          "name": "Personal iMessage",
          "allowed_ids": ["+8613800138000"]
        },
        "work": {
          "enabled": true,
          "name": "Work iMessage",
          "allowed_ids": ["work@company.com"]
        }
      }
    }
  }
}
```

---

## 工作原理

### 接收消息

1. 以只读模式打开 `~/Library/Messages/chat.db`（WAL mode）
2. 启动时记录当前最大 `ROWID`，仅处理后续新消息
3. 按轮询间隔查询 `ROWID > lastRowID` 的新消息
4. 过滤 `is_from_me = 0`，只处理收到的消息
5. 检查发送者是否在 `allowed_ids` 中
6. 转换为 `InboundMessage` 并发布到消息总线

### 发送消息

1. 通过 `os/exec` 执行 `osascript` 命令
2. AppleScript 调用 Messages.app 发送 iMessage
3. `chat_id` 为收件人的手机号或 iCloud 邮箱

---

## macOS 权限设置

首次使用时，macOS 可能会弹出权限请求对话框。需要授予以下权限：

1. **全盘访问权限**（读取 chat.db）
   - 系统设置 > 隐私与安全性 > 全盘磁盘访问权限 > 添加 goclaw

2. **自动化权限**（通过 AppleScript 控制 Messages.app）
   - 首次发送时系统会自动弹出请求

---

## 注意事项

- iMessage 通道仅在 macOS 上可用，在 Linux 上启用会返回错误
- Messages.app 必须处于运行状态才能发送消息
- `poll_interval` 建议不低于 2 秒，以避免频繁读取数据库
- macOS Messages 的时间戳使用 Core Data epoch（2001-01-01），已在代码中自动转换
