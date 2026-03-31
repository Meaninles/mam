# 项目代理说明

本项目已接入基于 `affaan-m/everything-claude-code` 的 Codex 本地配置。

## 语言要求

- 默认按中文软件编写所有用户可见内容，包括界面文案、提示语、错误信息、帮助说明和演示数据。
- 仅在以下情况保留英文或其他外语：专有名词、库名、接口字段、行业缩写，或任务明确要求使用其他语言。

## 工作方式

- 复杂任务先规划，再动手修改。
- 优先测试驱动；新增功能、修复问题、重构时先补或先写测试。
- 所有输入都在边界处校验，禁止硬编码密钥、令牌或密码。
- 结束前至少完成与改动相称的验证，例如构建、测试、lint、类型检查或关键路径自查。
- 文档优先写入项目既有结构；若没有明确归档位置，不要随意新增顶层说明文件。

## Codex 约定

- 使用项目内的 `[.codex/config.toml](B:\new_project\mare\.codex\config.toml)` 作为默认 Codex 配置。
- 使用 `[.codex/AGENTS.md](B:\new_project\mare\.codex\AGENTS.md)` 作为 Codex 补充说明。
- 可复用技能位于 `[.agents/skills](B:\new_project\mare\.agents\skills)`。
- 可选多代理角色位于 `[.codex/agents](B:\new_project\mare\.codex\agents)`。
