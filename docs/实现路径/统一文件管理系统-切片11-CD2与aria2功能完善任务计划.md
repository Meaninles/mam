# 统一文件管理系统-切片11-CD2与aria2功能完善任务计划

## 文档说明

- 更新时间：2026-04-12
- 适用范围：`client`、`services/center`、`shared/contracts`、`docs`
- 文档目标：在“前置切片均已完成”的前提下，把切片 11 直接收口为可进入生产环境的客户端页面任务计划
- 任务组织方式：严格按客户端页面顺序拆分，便于后续按多个独立任务逐步实现
- 约束来源：
  - 总纲真源：[统一文件管理系统-垂直切片实现路径.md](/B:/new_project/mare/docs/实现路径/统一文件管理系统-垂直切片实现路径.md)
  - 后端与集成总览：[统一文件管理系统-后端及核心服务总体设计方案.md](/B:/new_project/mare/docs/后端及核心服务/统一文件管理系统-后端及核心服务总体设计方案.md)
  - 当前缺陷结论：[统一文件管理系统-CD2远程上传只进缓存不进115的缺陷修复记录.md](/B:/new_project/mare/docs/bug修复/统一文件管理系统-CD2远程上传只进缓存不进115的缺陷修复记录.md)
  - 关键代码：
    - [App.tsx](/B:/new_project/mare/client/src/App.tsx)
    - [SettingsPanels.tsx](/B:/new_project/mare/client/src/pages/SettingsPanels.tsx)
    - [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx)
    - [FileCenterPage.tsx](/B:/new_project/mare/client/src/pages/FileCenterPage.tsx)
    - [TaskCenterWorkspace.tsx](/B:/new_project/mare/client/src/pages/TaskCenterWorkspace.tsx)
    - [IssuesPage.tsx](/B:/new_project/mare/client/src/pages/IssuesPage.tsx)
    - [NotificationCenterSheet.tsx](/B:/new_project/mare/client/src/pages/NotificationCenterSheet.tsx)
    - [ImportCenterPage.tsx](/B:/new_project/mare/client/src/pages/ImportCenterPage.tsx)
    - [services/center/internal/integration/service.go](/B:/new_project/mare/services/center/internal/integration/service.go)
    - [services/center/internal/storage/cloud_nodes.go](/B:/new_project/mare/services/center/internal/storage/cloud_nodes.go)
    - [services/center/internal/assets/heavy_ops_cloud.go](/B:/new_project/mare/services/center/internal/assets/heavy_ops_cloud.go)

## 1. 当前口径

### 1.1 切片 11 的正式目标

根据总纲，切片 11 的目标不是“补一点云盘 UI”，而是让系统真实接入 `115 / CloudDrive2 / aria2`，并使以下能力进入生产闭环：

- 115 接入配置
- CloudDrive2 编排
- aria2 传输接入
- 上传、下载类真实作业执行
- 云端副本回写
- 云端鉴权与异常回写

对应总纲位置：

- [统一文件管理系统-垂直切片实现路径.md](/B:/new_project/mare/docs/实现路径/统一文件管理系统-垂直切片实现路径.md):489
- [统一文件管理系统-垂直切片实现路径.md](/B:/new_project/mare/docs/实现路径/统一文件管理系统-垂直切片实现路径.md):500
- [统一文件管理系统-垂直切片实现路径.md](/B:/new_project/mare/docs/实现路径/统一文件管理系统-垂直切片实现路径.md):501
- [统一文件管理系统-垂直切片实现路径.md](/B:/new_project/mare/docs/实现路径/统一文件管理系统-垂直切片实现路径.md):502

### 1.2 本计划的前提

本计划不再把切片 1、4、5、7 当作阻塞条件。统一按“这些前置切片已经完成，可直接把切片 11 收口为生产环境能力”的口径组织任务。

这意味着：

- 不再拆“先做前置切片再做切片 11”
- 不再把“云端能力只是页面语义层”作为默认假设
- 任务重点转为“把已有真实能力按客户端页面收口为生产可用”

## 2. 当前实现基线

切片 11 当前并非从零开始，已经具备了较强的后端和局部客户端基础。

### 2.1 已落地的后端能力

- 集成服务已具备 `CloudDrive2` 网关保存、连接测试、运行态查询：
  - [service.go](/B:/new_project/mare/services/center/internal/integration/service.go):76
  - [service.go](/B:/new_project/mare/services/center/internal/integration/service.go):170
  - [service.go](/B:/new_project/mare/services/center/internal/integration/service.go):219
- 115 provider 已具备真实二维码会话、Token 鉴权、远程上传、下载地址解析、云端删除：
  - [provider_cd2_115.go](/B:/new_project/mare/services/center/internal/integration/provider_cd2_115.go)
- aria2 管理器已具备二进制准备、RPC 启动、下载入队、暂停、恢复、取消与运行态：
  - [downloader_aria2.go](/B:/new_project/mare/services/center/internal/integration/downloader_aria2.go)
- 资产域已经接入云端上传、云端下载、云端删副本、云端副本回写：
  - [heavy_ops_cloud.go](/B:/new_project/mare/services/center/internal/assets/heavy_ops_cloud.go):45
  - [heavy_ops_cloud.go](/B:/new_project/mare/services/center/internal/assets/heavy_ops_cloud.go):120
  - [heavy_ops_cloud.go](/B:/new_project/mare/services/center/internal/assets/heavy_ops_cloud.go):205
  - [heavy_ops_cloud.go](/B:/new_project/mare/services/center/internal/assets/heavy_ops_cloud.go):212
- 应用启动时已注册 `115 provider + aria2 downloader + REPLICATE 作业执行器`：
  - [app.go](/B:/new_project/mare/services/center/internal/app/app.go):56
  - [app.go](/B:/new_project/mare/services/center/internal/app/app.go):57
  - [app.go](/B:/new_project/mare/services/center/internal/app/app.go):82

### 2.2 已落地的客户端能力

- 设置页已能显示 CloudDrive2 集成卡片，并执行保存、测试、运行状态展示：
  - [SettingsPanels.tsx](/B:/new_project/mare/client/src/pages/SettingsPanels.tsx):104
  - [SettingsPanels.tsx](/B:/new_project/mare/client/src/pages/SettingsPanels.tsx):162
  - [SettingsPanels.tsx](/B:/new_project/mare/client/src/pages/SettingsPanels.tsx):165
  - [SettingsPanels.tsx](/B:/new_project/mare/client/src/pages/SettingsPanels.tsx):169
- 存储节点页已能保存网盘节点、创建二维码会话、轮询二维码状态、运行网盘连接测试：
  - [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx):443
  - [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx):547
  - [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx):1821
  - [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx):1883
  - [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx):1916
- 任务中心已能识别 `CD2_REMOTE_UPLOAD` 和 `ARIA2` 外部状态，并翻译成中文阶段文案：
  - [TaskCenterWorkspace.tsx](/B:/new_project/mare/client/src/pages/TaskCenterWorkspace.tsx):525
  - [TaskCenterWorkspace.tsx](/B:/new_project/mare/client/src/pages/TaskCenterWorkspace.tsx):530
  - [TaskCenterWorkspace.tsx](/B:/new_project/mare/client/src/pages/TaskCenterWorkspace.tsx):542
- App 已在启动时加载集成网关与运行态：
  - [App.tsx](/B:/new_project/mare/client/src/App.tsx):1132
  - [App.tsx](/B:/new_project/mare/client/src/App.tsx):1135
  - [App.tsx](/B:/new_project/mare/client/src/App.tsx):1136

### 2.3 当前仍未收口的问题

虽然真实能力已经存在，但客户端页面还没有完全达到生产收口状态，主要问题集中在：

- 页面展示和后端能力不一致，仍有旧文档口径残留
- 页面缺少“集成运行态 -> 动作禁用/告警/跳转”的前置反馈
- 云端任务在任务中心已能识别状态，但外部执行器信息展示不足
- 异常中心、通知中心对云端运行态问题的治理入口还不够强
- 导入中心对 `CLOUD` 目标端仍未正式开放

## 3. 页面任务总览

按推荐实施顺序，切片 11 的客户端页面任务如下：

1. 设置页：集成总控面板收口
2. 存储节点页：115 节点鉴权与可见性收口
3. 文件中心页：云端同步/删副本动作收口
4. 任务中心页：外部执行器可观测性收口
5. 异常中心页：云端异常治理入口收口
6. 通知中心页：云端运行提醒与跳转收口
7. 导入中心页：CLOUD 目标端导入编排收口

说明：

- 顺序按客户端用户路径组织，而不是按服务模块组织
- 每个页面任务都可以作为单独一次 vibe coding 实现
- 每个页面任务完成后都应该形成一条最小人工验收路径

## 4. 任务一：设置页

### 4.1 页面定位

设置页是切片 11 的第一个入口页，用于收口“集成是否可用”的系统级前置判断。

当前代码事实：

- 页面已展示 `CloudDrive2` + `aria2` 双组件运行态，并提供运行状态汇总卡片
- App 启动时同时加载 `gateways` 与 `runtime`
- CloudDrive2 支持保存与连接测试，aria2 由中心服务托管仅展示运行状态

### 4.2 当前还差哪些工作

- 依赖服务页已具备双组件展示与运行状态汇总
- 已展示最近检测时间、错误摘要、是否已保存凭据等运维信息
- 集成健康状态已由 App 汇总并提供给文件中心动作门禁
- 当前未提供“禁用集成”的独立开关（代码未接入）

### 4.3 任务范围

- [App.tsx](/B:/new_project/mare/client/src/App.tsx)
- [SettingsPanels.tsx](/B:/new_project/mare/client/src/pages/SettingsPanels.tsx)
- [integrationsApi.ts](/B:/new_project/mare/client/src/lib/integrationsApi.ts)

### 4.4 建议拆分

1. 补齐 `aria2` 运行态读取与展示
2. 补齐 `hasPassword / lastErrorCode / lastErrorMessage` 的前端消费
3. 把设置页中的集成卡片升级为“集成总控卡片”
4. 明确区分“未配置 / 已配置未检测 / 在线 / 异常 / 已禁用”

### 4.5 完成标志

- 用户能在设置页确认 `CloudDrive2` 是否在线
- 用户能在设置页确认 `aria2` 是否在线
- 用户能看见最近失败原因，而不是只能看到“异常”
- 后续页面可以复用统一的集成健康状态

## 5. 任务二：存储节点页

### 5.1 页面定位

存储节点页是切片 11 中 115 配置和鉴权的正式工作台。

当前代码事实：

- 已支持保存网盘节点
- 已支持 `Token` 和扫码登录两种入口
- 已支持二维码创建、轮询、连接测试
- 但网盘列表字段和详情信息仍不足以支撑生产排障

### 5.2 当前还差哪些工作

- 列表中补齐 `Token 状态` 的正式展示
- 补齐账号别名、鉴权失败原因、最近鉴权结果等真实信息
- 编辑弹层中补齐“当前是否已有有效凭据”的反馈
- 批量连接测试和单项连接测试结果要统一映射成更清晰的用户口径
- 收口旧文档中“没有真实 NAS / 网盘鉴权全链路完全收口”的过时口径

### 5.3 任务范围

- [StorageNodesPage.tsx](/B:/new_project/mare/client/src/pages/StorageNodesPage.tsx)
- [storageNodesApi.ts](/B:/new_project/mare/client/src/lib/storageNodesApi.ts)
- [cloud_nodes.go](/B:/new_project/mare/services/center/internal/storage/cloud_nodes.go)

### 5.4 建议拆分

1. 补齐云节点表格字段，至少把 `Token 状态` 正式显示出来
2. 补齐编辑态“已保存凭据 / 需要重新登录 / 最近失败原因”提示
3. 收口二维码登录后的完成反馈与错误反馈
4. 收口连接测试结果文案与状态色

### 5.5 完成标志

- 用户能在网盘列表中判断“节点已配置但未鉴权”还是“鉴权已失效”
- 用户能在编辑网盘时判断是否需要重新扫码或重填 Token
- 用户能通过连接测试结果快速定位问题是在 115、CD2 还是目录根路径校验

## 6. 任务三：文件中心页

### 6.1 页面定位

文件中心是切片 11 面向最终业务动作的主工作台，承担“同步到 115 / 从 115 下载 / 删除 115 副本”的实际用户入口。

当前代码事实：

- 文件中心的重操作入口已经都走正式接口
- 后端已经支持 `UPLOAD / DOWNLOAD / CLOUD DELETE`，并接入 CloudDrive2/aria2 云端链路
- 前端动作判断已纳入集成运行态（`cd2Online` / `aria2Online` / `cloudAuthReady`）
- 页面会在云端动作受限时提示并提供跳转入口

### 6.2 当前还差哪些工作

- `CLOUD -> CLOUD` 同步与离线下载 + 自动转存仍未支持
- 云端问题的异常分类与治理入口仍需在异常中心强化

### 6.3 任务范围

- [FileCenterPage.tsx](/B:/new_project/mare/client/src/pages/FileCenterPage.tsx)
- [fileCenterApi.ts](/B:/new_project/mare/client/src/lib/fileCenterApi.ts)
- [App.tsx](/B:/new_project/mare/client/src/App.tsx)

### 6.4 建议拆分

1. 为文件中心注入全局集成健康状态
2. 收口 `115` 端点的同步、下载、删副本文案与禁用逻辑
3. 对云端失败结果补齐更明确的提示和跳转
4. 刷新文件中心页面文档

### 6.5 完成标志

- 用户在文件中心能明确知道当前发起的是“上传到 115”还是“从 115 下载”
- 当 `CloudDrive2` 或 `aria2` 不可用时，文件中心不会给出误导性的可执行状态
- 文件中心文档与真实代码口径一致

## 7. 任务四：任务中心页

### 7.1 页面定位

任务中心是切片 11 的实际执行观测台。

当前代码事实：

- 已能识别 `CD2_REMOTE_UPLOAD`
- 已能识别 `ARIA2`
- 已能把外部状态翻译为“上传中 / 下载中 / 上传失败 / 下载失败”

### 7.2 当前还差哪些工作

- 任务详情与子项详情已展示执行引擎、外部任务 ID、外部状态与失败位置
- 已区分中心服务失败与外部执行器失败，并保留原始错误信息

### 7.3 任务范围

- [TaskCenterWorkspace.tsx](/B:/new_project/mare/client/src/pages/TaskCenterWorkspace.tsx)
- [jobsApi.ts](/B:/new_project/mare/client/src/lib/jobsApi.ts)
- 必要时补 DTO 映射

### 7.4 建议拆分

1. 在详情区补“执行引擎 / 外部任务状态 / 外部任务编号”
2. 在列表或子项行增加对外部阶段的更显式提示
3. 对失败任务补“失败位置”语义，例如“CD2 上传失败”或“aria2 下载失败”
4. 收口任务中心文档的旧边界描述

### 7.5 完成标志

- 用户能在任务中心直接看懂云端任务当前卡在哪一段
- 运维或开发排障时不必再依赖服务端日志才能判断外部链路故障

## 8. 任务五：异常中心页

### 8.1 页面定位

异常中心是切片 11 的统一治理入口，负责把云端能力中的失败和风险收口成可治理对象。

当前代码事实：

- 异常中心已经支持任务、文件、存储节点上下文跳转
- 但云端异常还没有形成足够强的独立分类与治理入口

### 8.2 当前还差哪些工作

- 把云端传输问题明确分成几类：
  - 115 鉴权问题
  - CloudDrive2 网关问题
  - aria2 下载器问题
  - 云端路径或目录问题
  - 传输执行问题
- 在异常详情中补齐云端上下文
- 明确从异常中心跳设置页、存储节点页或任务中心的优先路径

### 8.3 任务范围

- [IssuesPage.tsx](/B:/new_project/mare/client/src/pages/IssuesPage.tsx)
- [issuesApi.ts](/B:/new_project/mare/client/src/lib/issuesApi.ts)
- 必要时补服务端 issue 投影逻辑

### 8.4 建议拆分

1. 补齐云端异常分类与标签
2. 补齐云端异常详情中的执行器上下文
3. 补齐从异常到设置页、存储节点页、任务中心的精确跳转策略
4. 收口异常中心文档口径

### 8.5 完成标志

- 云端问题不再只是“普通失败任务”
- 用户可以直接从异常中心判断这是鉴权问题、下载器问题还是传输问题

## 9. 任务六：通知中心页

### 9.1 页面定位

通知中心是切片 11 的轻提醒入口，用于把云端运行风险及时推到用户面前。

### 9.2 当前还差哪些工作

- 把云端运行态问题纳入正式通知来源
- 区分“需要处理的云端异常”和“仅提醒的运行波动”
- 为 `CD2 离线 / aria2 未就绪 / 115 鉴权失效 / 云端目录校验失败` 提供准确跳转

### 9.3 任务范围

- [NotificationCenterSheet.tsx](/B:/new_project/mare/client/src/pages/NotificationCenterSheet.tsx)
- [notificationsApi.ts](/B:/new_project/mare/client/src/lib/notificationsApi.ts)
- 必要时补服务端通知投影逻辑

### 9.4 建议拆分

1. 明确云端运行态通知模型
2. 为通知增加更准确的跳转目标
3. 控制去重策略，避免同一云端问题短时间内重复刷屏
4. 收口通知中心文档口径

### 9.5 完成标志

- 云端问题出现时，用户不必靠手动巡检页面才能发现
- 通知能直接把用户带到正确处置页面

## 10. 任务七：导入中心页

### 10.1 页面定位

导入中心是切片 10 与切片 11 的交叉收口页。它不是切片 11 的第一优先级，但在切片 11 最终进入生产环境前必须补齐。

### 10.2 当前还差哪些工作

- 正式开放 `CLOUD` 目标端导入编排
- 明确哪些导入目标走本地复制，哪些走云端上传
- 把提交导入后的云端结果正确回写到任务中心、文件中心、异常中心、通知中心
- 刷新导入中心文档中的“NAS / CLOUD 目标正式导入执行未完成”口径

### 10.3 任务范围

- [ImportCenterPage.tsx](/B:/new_project/mare/client/src/pages/ImportCenterPage.tsx)
- [importsApi.ts](/B:/new_project/mare/client/src/lib/importsApi.ts)
- 导入域服务端相关编排

### 10.4 建议拆分

1. 让导入中心真实暴露可用的 `CLOUD` 目标端
2. 收口提交前校验与目标端可用性判断
3. 收口提交后云端结果的跨页回写
4. 刷新导入中心页面文档

### 10.5 完成标志

- 用户能在导入中心把内容正式导入到云端目标
- 导入结果在任务、文件、异常、通知四个页面中都能一致体现

## 11. 跨页收尾要求

虽然本计划按页面组织，但切片 11 进入生产环境前，必须补做以下跨页收尾：

- 更新相关页面设计文档，消除旧口径
- 补齐与页面相匹配的自动化测试
- 至少完成一条真实烟测路径：
  - 配置 CD2
  - 配置 115 节点
  - 从文件中心发起上传到 115
  - 在任务中心观察 `CD2_REMOTE_UPLOAD`
  - 在文件中心看到云端副本回写
  - 失败场景能进入异常中心和通知中心
- 至少完成一条真实下载烟测路径：
  - 从 115 下载到 `LOCAL` 或 `NAS`
  - 在任务中心观察 `ARIA2`
  - 下载成功后看到本地副本写回

## 12. 推荐执行节奏

建议按以下 7 次独立任务推进：

1. 设置页
2. 存储节点页
3. 文件中心页
4. 任务中心页
5. 异常中心页
6. 通知中心页
7. 导入中心页

推荐原因：

- 先把“集成是否可用”收口
- 再把“115 节点是否可用”收口
- 再把“业务动作是否可执行”和“执行是否可观测”收口
- 最后再补跨页治理和导入编排

## 13. 结论

切片 11 当前缺的已经不是“后端能不能接 CD2 / aria2”，而是客户端页面还没有完全把这些真实能力收口为生产级体验。

因此，后续实现不应再按“写一个新的 integration 模块”来组织，而应按客户端页面顺序推进：

- 先设置页总控
- 再存储节点页配置
- 再文件中心动作
- 再任务中心观测
- 再异常中心治理
- 再通知中心提醒
- 最后补导入中心云目标编排

只要按这个顺序推进，切片 11 就可以从“已有真实后端能力的半收口状态”进入“客户端可正式投产的完整状态”。
